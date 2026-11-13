package sftpgateway

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	glssh "github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

var ErrInvalidGatewayConfig = errors.New("invalid sftp gateway configuration")

type Options struct {
	Listen      string
	HostKey     ssh.Signer
	LandingZone string
	IdleTimeout time.Duration
	Auth        TokenValidator
	Control     TransferControlPort
}

type Server struct {
	server *glssh.Server
}

func New(opts Options) (*Server, error) {
	if opts.Listen == "" {
		return nil, fmt.Errorf("%w: empty listen address", ErrInvalidGatewayConfig)
	}
	if opts.LandingZone == "" {
		return nil, fmt.Errorf("%w: empty landing zone", ErrInvalidGatewayConfig)
	}
	if opts.HostKey == nil {
		return nil, fmt.Errorf("%w: host key required", ErrInvalidGatewayConfig)
	}
	if opts.Auth == nil {
		opts.Auth = UnconfiguredTokenValidator{}
	}
	if opts.Control == nil {
		opts.Control = UnconfiguredTransferControl{}
	}
	landingAbs, err := filepath.Abs(opts.LandingZone)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(landingAbs, 0o750); err != nil {
		return nil, err
	}
	idle := opts.IdleTimeout
	if idle == 0 {
		idle = 15 * time.Minute
	}

	s := &Server{}
	auth := opts.Auth
	control := opts.Control
	s.server = &glssh.Server{
		Addr:        opts.Listen,
		IdleTimeout: idle,
		MaxTimeout:  idle,
		HostSigners: []glssh.Signer{opts.HostKey},
		PasswordHandler: func(ctx glssh.Context, password string) bool {
			principal, err := auth.ValidateSFTPPassword(ctx, ctx.User(), password)
			if err != nil {
				return false
			}
			ctx.SetValue(sessionPrincipalKey{}, principal)
			return true
		},
		Handler: func(sess glssh.Session) {
			_, _ = io.WriteString(sess, "sftp subsystem required\n")
		},
		SubsystemHandlers: map[string]glssh.SubsystemHandler{
			"sftp": sftpSubsystem(landingAbs, control),
		},
	}
	return s, nil
}

func (s *Server) ListenAndServe() error {
	if s == nil || s.server == nil {
		return fmt.Errorf("sftp server: not initialized")
	}
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func sftpSubsystem(landingZone string, control TransferControlPort) glssh.SubsystemHandler {
	return func(sess glssh.Session) {
		principal, err := principalFromSession(sess)
		if err != nil {
			_, _ = io.WriteString(sess.Stderr(), err.Error())
			return
		}
		sessionID := uuid.New()
		sessionRoot, err := SessionLandingDir(landingZone, principal, sessionID)
		if err != nil {
			_, _ = io.WriteString(sess.Stderr(), err.Error())
			return
		}
		if err := os.MkdirAll(sessionRoot, 0o750); err != nil {
			_, _ = io.WriteString(sess.Stderr(), err.Error())
			return
		}
		vfs, err := NewVirtualFS(sessionRoot)
		if err != nil {
			_, _ = io.WriteString(sess.Stderr(), err.Error())
			return
		}
		importer := NewLandingImporter(control, principal, sessionRoot)
		server := sftp.NewRequestServer(sess, newSessionHandler(vfs, importer))
		if err := server.Serve(); err != nil && !errors.Is(err, io.EOF) {
			_, _ = io.WriteString(sess.Stderr(), err.Error())
		}
		_ = server.Close()
	}
}

type sessionPrincipalKey struct{}

func principalFromSession(sess glssh.Session) (Principal, error) {
	if v := sess.Context().Value(sessionPrincipalKey{}); v != nil {
		if p, ok := v.(Principal); ok && p.bound() {
			return p, nil
		}
	}
	return Principal{}, ErrAuthFailed
}

func NewFromConfig(cfg *config.Config) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: nil config", ErrInvalidGatewayConfig)
	}
	sftpCfg := cfg.Filesystem.SFTP
	if !sftpCfg.Enabled {
		return nil, fmt.Errorf("%w: sftp gateway disabled", ErrInvalidGatewayConfig)
	}
	if pathsOverlap(sftpCfg.LandingZone, cfg.Filesystem.TransferGateway.StorageRoot) {
		return nil, fmt.Errorf("%w: landing zone overlaps provider storage", ErrInvalidGatewayConfig)
	}
	signer, err := LoadHostKey(sftpCfg.HostKeySecretRef)
	if err != nil {
		return nil, err
	}
	return New(Options{
		Listen:      sftpCfg.Listen,
		HostKey:     signer,
		LandingZone: sftpCfg.LandingZone,
		IdleTimeout: sftpCfg.IdleTimeout,
		Auth:        UnconfiguredTokenValidator{},
		Control:     UnconfiguredTransferControl{},
	})
}

// LoadHostKey resolves the host key PEM from env or file ref.
func LoadHostKey(secretRef string) (ssh.Signer, error) {
	if pemBytes := strings.TrimSpace(os.Getenv("SPARKOPS_SFTP_HOST_KEY_PEM")); pemBytes != "" {
		return parseHostKeyPEM([]byte(pemBytes))
	}
	ref := strings.TrimSpace(secretRef)
	if strings.HasPrefix(ref, "file://") {
		path := strings.TrimPrefix(ref, "file://")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("host key file: %w", err)
		}
		return parseHostKeyPEM(data)
	}
	if ref == "" {
		return generateEphemeralHostKey()
	}
	return nil, fmt.Errorf("host key secret ref %q not resolved; set SPARKOPS_SFTP_HOST_KEY_PEM", ref)
}

func parseHostKeyPEM(pemBytes []byte) (ssh.Signer, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("invalid host key pem")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return ssh.NewSignerFromKey(key)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		key, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse host key: %w", err)
		}
	}
	return ssh.NewSignerFromKey(key)
}

func generateEphemeralHostKey() (ssh.Signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return ssh.NewSignerFromKey(key)
}
