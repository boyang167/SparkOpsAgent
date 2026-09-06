package jenkins

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	gjenkins "github.com/bndr/gojenkins"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Executor struct {
	client    *gjenkins.Jenkins
	jobSuffix string
}

func NewExecutor(cfg config.JenkinsConfig) (*Executor, error) {
	return NewExecutorFromConnection(ConnectionConfig{
		URL: cfg.URL, User: cfg.User, Token: resolveToken(cfg.Token, ""),
	}, cfg.JobSuffix)
}

type ConnectionConfig struct {
	URL      string
	User     string
	Password string
	Token    string
}

func NewExecutorFromConnection(conn ConnectionConfig, jobSuffix string) (*Executor, error) {
	url := strings.TrimSpace(conn.URL)
	if url == "" {
		return nil, fmt.Errorf("jenkins url is required")
	}
	token := resolveToken(conn.Token, conn.Password)
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("jenkins token is required")
	}
	client := gjenkins.CreateJenkins(nil, url, strings.TrimSpace(conn.User), token)
	if _, err := client.Init(context.Background()); err != nil {
		return nil, fmt.Errorf("jenkins init: %w", err)
	}
	if jobSuffix == "" {
		jobSuffix = "-build"
	}
	return &Executor{client: client, jobSuffix: jobSuffix}, nil
}

func resolveToken(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (e *Executor) CreateOrUpdate(ctx context.Context, spec port.JenkinsJobSpec) error {
	spec.Folder = strings.TrimSpace(spec.Folder)
	spec.Job = strings.TrimSpace(spec.Job)
	if spec.Folder == "" {
		return errors.New("jenkins folder is required")
	}
	if spec.Job == "" {
		return errors.New("jenkins job is required")
	}

	if err := e.ensureFolder(ctx, spec.Folder); err != nil {
		return err
	}
	if err := e.upsertCredentials(ctx, spec.Folder, spec.Credentials); err != nil {
		return err
	}

	config := renderJobXML(spec)
	jobBase := "/job/" + url.PathEscape(spec.Folder) + "/job/" + url.PathEscape(spec.Job)
	jobStatus, err := e.resourceStatus(ctx, jobBase)
	if err != nil {
		return fmt.Errorf("get jenkins job: %w", err)
	}
	if jobStatus == http.StatusNotFound {
		if _, err := e.client.CreateJobInFolder(ctx, config, spec.Job, spec.Folder); err != nil {
			if !isCreateConflict(err) {
				return fmt.Errorf("create jenkins job: %w", err)
			}
			reloadedStatus, reloadErr := e.resourceStatus(ctx, jobBase)
			if reloadErr != nil {
				return fmt.Errorf("reload jenkins job after create conflict: %w", reloadErr)
			}
			if reloadedStatus != http.StatusOK {
				return fmt.Errorf("reload jenkins job after create conflict: unexpected status %d", reloadedStatus)
			}
			return e.updateJobConfig(ctx, spec.Folder, spec.Job, config)
		}
		return nil
	}
	if jobStatus != http.StatusOK {
		return fmt.Errorf("get jenkins job: unexpected status %d", jobStatus)
	}
	return e.updateJobConfig(ctx, spec.Folder, spec.Job, config)
}

func (e *Executor) SyncCredentials(ctx context.Context, folder string, credentials []port.JenkinsCredential) error {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return errors.New("jenkins folder is required")
	}
	if err := e.ensureFolder(ctx, folder); err != nil {
		return err
	}
	return e.upsertCredentials(ctx, folder, credentials)
}

func (e *Executor) ensureFolder(ctx context.Context, folder string) error {
	folderStatus, err := e.resourceStatus(ctx, "/job/"+url.PathEscape(folder))
	if err != nil {
		return fmt.Errorf("get jenkins folder: %w", err)
	}
	if folderStatus == http.StatusNotFound {
		if _, err := e.client.CreateFolder(ctx, folder); err != nil {
			if !isCreateConflict(err) {
				return fmt.Errorf("create jenkins folder: %w", err)
			}
			reloadedStatus, reloadErr := e.resourceStatus(ctx, "/job/"+url.PathEscape(folder))
			if reloadErr != nil {
				return fmt.Errorf("reload jenkins folder after create conflict: %w", reloadErr)
			}
			if reloadedStatus != http.StatusOK {
				return fmt.Errorf("reload jenkins folder after create conflict: unexpected status %d", reloadedStatus)
			}
		}
		return nil
	}
	if folderStatus != http.StatusOK {
		return fmt.Errorf("get jenkins folder: unexpected status %d", folderStatus)
	}
	return nil
}

func (e *Executor) updateJobConfig(ctx context.Context, folder, jobName, config string) error {
	job, err := e.client.GetJob(ctx, jobName, folder)
	if err != nil {
		return fmt.Errorf("get jenkins job: %w", err)
	}
	if err := job.UpdateConfig(ctx, config); err != nil {
		return fmt.Errorf("update jenkins job: %w", err)
	}
	return nil
}

func isCreateConflict(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return message == strconv.Itoa(http.StatusConflict) ||
		strings.Contains(message, "already exists") ||
		strings.Contains(message, "conflict status")
}

func (e *Executor) resourceStatus(ctx context.Context, base string) (int, error) {
	var body string
	response, err := e.client.Requester.Get(ctx, base+"/api/json", &body, nil)
	if err != nil {
		return 0, err
	}
	return response.StatusCode, nil
}

func (e *Executor) upsertCredentials(ctx context.Context, folder string, credentials []port.JenkinsCredential) error {
	manager := gjenkins.CredentialsManager{J: e.client, Folder: folder}
	existing, err := manager.List(ctx, "_")
	if err != nil {
		return fmt.Errorf("list jenkins credentials: %w", err)
	}
	exists := make(map[string]bool, len(existing))
	for _, id := range existing {
		exists[id] = true
	}
	for _, credential := range credentials {
		value, err := jenkinsCredentialValue(credential)
		if err != nil {
			return fmt.Errorf("prepare jenkins credential %q: %w", credential.ID, err)
		}
		if exists[credential.ID] {
			err = manager.Update(ctx, "_", credential.ID, value)
		} else {
			err = manager.Add(ctx, "_", value)
			if isCreateConflict(err) {
				reloaded, reloadErr := manager.List(ctx, "_")
				if reloadErr != nil {
					return fmt.Errorf("reload jenkins credential %q after create conflict: %w", credential.ID, reloadErr)
				}
				found := false
				for _, id := range reloaded {
					if id == credential.ID {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("reload jenkins credential %q after create conflict: credential not found", credential.ID)
				}
				err = manager.Update(ctx, "_", credential.ID, value)
			}
		}
		if err != nil {
			return fmt.Errorf("upsert jenkins credential %q: %w", credential.ID, err)
		}
	}
	return nil
}

func jenkinsCredentialValue(credential port.JenkinsCredential) (any, error) {
	const scope = "GLOBAL"
	switch credential.Kind {
	case "username-password":
		return gjenkins.UsernameCredentials{
			ID: credential.ID, Scope: scope, Username: credential.Username, Password: credential.Secret,
		}, nil
	case "secret-text":
		return gjenkins.StringCredentials{
			ID: credential.ID, Scope: scope, Secret: credential.Secret,
		}, nil
	case "ssh-private-key":
		return gjenkins.SSHCredentials{
			ID: credential.ID, Scope: scope, Username: credential.Username,
			PrivateKeySource: &gjenkins.PrivateKey{
				Value: credential.Secret,
				Class: gjenkins.KeySourceDirectEntryType,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported kind %q", credential.Kind)
	}
}

func renderJobXML(spec port.JenkinsJobSpec) string {
	var script bytes.Buffer
	_ = xml.EscapeText(&script, []byte(spec.PipelineScript))
	return `<?xml version="1.0" encoding="UTF-8"?>` +
		`<flow-definition plugin="workflow-job">` +
		`<actions/><description/><keepDependencies>false</keepDependencies>` +
		`<properties><hudson.model.ParametersDefinitionProperty><parameterDefinitions>` +
		stringParameterXML("BRANCH") +
		stringParameterXML("CodeVersion") +
		stringParameterXML("IMAGE_TAG") +
		stringParameterXML("IMAGE_REPOSITORY") +
		`</parameterDefinitions></hudson.model.ParametersDefinitionProperty></properties>` +
		`<definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps">` +
		`<script>` + script.String() + `</script><sandbox>true</sandbox></definition>` +
		`<triggers/><disabled>false</disabled></flow-definition>`
}

func stringParameterXML(name string) string {
	return `<hudson.model.StringParameterDefinition><name>` + name +
		`</name><defaultValue></defaultValue><trim>true</trim></hudson.model.StringParameterDefinition>`
}

func buildParams(req port.BuildRequest) map[string]string {
	branch := strings.TrimSpace(req.GitRefValue)
	if branch == "" {
		branch = "main"
	}
	params := map[string]string{"BRANCH": branch}
	if req.Namespace != "" || req.ProjectName != "" {
		params["CodeVersion"] = branch
		params["IMAGE_TAG"] = req.ImageTag
		params["IMAGE_REPOSITORY"] = req.ImageRepository
	}
	if req.RollbackOf > 0 {
		params["ROLLBACK_OF"] = strconv.Itoa(req.RollbackOf)
	}
	return params
}

func (e *Executor) RunStreaming(
	ctx context.Context,
	buildID kernel.BuildID,
	req port.BuildRequest,
	hub port.LogPublisher,
) (string, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}
	req.Job = strings.TrimSpace(req.Job)
	if req.Job == "" {
		req.Job = "app" + e.jobSuffix
	}
	if strings.TrimSpace(req.GitRefValue) == "" {
		req.GitRefValue = "main"
	}
	header := fmt.Sprintf("[jenkins] job=%s branch=%s\n", req.Job, req.GitRefValue)
	if hub != nil {
		hub.Publish(buildID, header)
	}
	var buf strings.Builder
	buf.WriteString(header)

	var job *gjenkins.Job
	var err error
	if strings.TrimSpace(req.Folder) == "" {
		job, err = e.client.GetJob(ctx, req.Job)
	} else {
		job, err = e.client.GetJob(ctx, req.Job, strings.TrimSpace(req.Folder))
	}
	if err != nil {
		errLine := fmt.Sprintf("jenkins job not found: %s (%v)\n", req.Job, err)
		buf.WriteString(errLine)
		if hub != nil {
			hub.Publish(buildID, errLine)
		}
		return buf.String(), err
	}

	queueID, err := job.InvokeSimple(ctx, buildParams(req))
	if err != nil {
		return buf.String(), fmt.Errorf("invoke jenkins: %w", err)
	}
	build, err := e.client.GetBuildFromQueueID(ctx, job, queueID)
	if err != nil {
		return buf.String(), fmt.Errorf("wait build: %w", err)
	}

	var offset int64
	for {
		select {
		case <-ctx.Done():
			return buf.String(), ctx.Err()
		default:
		}

		if _, err := build.Poll(ctx); err != nil {
			return buf.String(), fmt.Errorf("poll jenkins build: %w", err)
		}

		offset, hasMore, err := e.publishConsoleChunk(ctx, build, buildID, hub, &buf, offset)
		if err != nil {
			return buf.String(), err
		}

		if !build.Raw.Building {
			for hasMore {
				offset, hasMore, err = e.publishConsoleChunk(ctx, build, buildID, hub, &buf, offset)
				if err != nil {
					return buf.String(), err
				}
			}
			break
		}

		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return buf.String(), ctx.Err()
		case <-timer.C:
		}
		next, err := job.GetBuild(ctx, build.GetBuildNumber())
		if err != nil {
			return buf.String(), fmt.Errorf("refresh jenkins build: %w", err)
		}
		build = next
	}

	result := fmt.Sprintf("\n[jenkins] finished result=%s\n", build.GetResult())
	buf.WriteString(result)
	if hub != nil {
		hub.Publish(buildID, result)
	}
	if build.GetResult() != "SUCCESS" {
		return buf.String(), fmt.Errorf("jenkins result: %s", build.GetResult())
	}
	return buf.String(), nil
}

func (e *Executor) publishConsoleChunk(
	ctx context.Context,
	build *gjenkins.Build,
	buildID kernel.BuildID,
	hub port.LogPublisher,
	buf *strings.Builder,
	offset int64,
) (int64, bool, error) {
	resp, err := build.GetConsoleOutputFromIndex(ctx, offset)
	if err != nil {
		return offset, false, fmt.Errorf("read jenkins console: %w", err)
	}
	if resp.Content != "" {
		buf.WriteString(resp.Content)
		if hub != nil {
			hub.Publish(buildID, resp.Content)
		}
	}
	return resp.Offset, resp.HasMoreText, nil
}

var _ port.JenkinsExecutor = (*Executor)(nil)
