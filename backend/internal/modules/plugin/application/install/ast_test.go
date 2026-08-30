package install

import "testing"

func TestFileDeclaresRegisterEngine(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "method with gin engine parameter",
			src: `package example

import "github.com/gin-gonic/gin"

type plugin struct{}

func (p *plugin) Register(engine *gin.Engine) {}
`,
			want: true,
		},
		{
			name: "function is not accepted",
			src: `package example

import "github.com/gin-gonic/gin"

func Register(engine *gin.Engine) {}
`,
			want: false,
		},
		{
			name: "method without parameters is not accepted",
			src: `package example

type plugin struct{}

func (p *plugin) Register() {}
`,
			want: false,
		},
		{
			name: "method with non pointer parameter is not accepted",
			src: `package example

import "github.com/gin-gonic/gin"

type plugin struct{}

func (p *plugin) Register(engine gin.Engine) {}
`,
			want: false,
		},
		{
			name: "method with extra parameter is not accepted",
			src: `package example

import "github.com/gin-gonic/gin"

type plugin struct{}

func (p *plugin) Register(engine *gin.Engine, other string) {}
`,
			want: false,
		},
		{
			name: "method with two named parameters in one field is not accepted",
			src: `package example

import "github.com/gin-gonic/gin"

type plugin struct{}

func (p *plugin) Register(a, b *gin.Engine) {}
`,
			want: false,
		},
		{
			name: "method with fake gin import is not accepted",
			src: `package example

import gin "example.com/fake/gin"

type plugin struct{}

func (p *plugin) Register(engine *gin.Engine) {}
`,
			want: false,
		},
		{
			name: "invalid go is not accepted",
			src:  `package example func`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FileDeclaresRegisterEngine([]byte(tt.src)); got != tt.want {
				t.Fatalf("FileDeclaresRegisterEngine() = %v, want %v", got, tt.want)
			}
		})
	}
}
