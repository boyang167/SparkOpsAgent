package query

import (
	"strings"
)

const builtinJenkinsfileVersionMarker = "SparkOps CI"

// BuiltinJenkinsfileTemplateDetail returns the Jenkinsfile template library source
// with Go-template placeholders for project/runtime variables.
func BuiltinJenkinsfileTemplateDetail() string {
	return `pipeline {
  agent any

  options {
    disableConcurrentBuilds()
    timeout(time: 90, unit: 'MINUTES')
  }

  environment {
    APP_NAME               = '{{.name}}'
    PROJECT_NAME           = '{{.name}}'
    NAMESPACE              = '{{.namespace}}'
    WORKSPACE_SLUG         = '{{.workspace_slug}}'
    ORG_SLUG               = '{{.org_slug}}'
    LANGUAGE               = '{{.language}}'
    GIT_URL                = '{{.git_url}}'
    GIT_REF_TYPE           = '{{.git_ref_type}}'
    DEFAULT_GIT_REF        = '{{.git_ref_value}}'
    DOCKERFILE_PATH        = '{{.dockerfile_path}}'
    REGISTRY_HOST          = '{{.registry_host}}'
    REGISTRY_TYPE          = '{{.registry_type}}'
    REGISTRY_URL           = '{{.registry_url}}'
    DEFAULT_IMAGE          = '{{.image}}'
    GIT_CREDENTIAL_ID      = '{{.git_credential_id}}'
    REGISTRY_CREDENTIAL_ID = '{{.registry_credential_id}}'
    IMAGE_PULL_SECRET      = '{{.image_pull_secret_name}}'
  }

  stages {
    stage('Prepare') {
      steps {
        script {
          env.BRANCH = (params.BRANCH ?: env.DEFAULT_GIT_REF ?: 'main')?.trim() ?: 'main'
          env.CODE_VERSION = (params.CodeVersion ?: env.BRANCH)?.trim() ?: env.BRANCH
          env.IMAGE_TAG = (params.IMAGE_TAG ?: 'latest')?.trim()
          env.IMAGE_REPOSITORY = (params.IMAGE_REPOSITORY ?: '{{.registry_host}}/{{.workspace_slug}}/{{.name}}')?.trim()
          env.FULL_IMAGE = "${env.IMAGE_REPOSITORY}:${env.IMAGE_TAG}"
          def registryURL = (env.REGISTRY_URL ?: '').trim()
          def registryHost = (env.REGISTRY_HOST ?: '').trim()
          def privateRegistryHost = registryHost ==~ /^(10\.\d{1,3}\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}\.\d{1,3}|172\.(1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}|localhost|127\.0\.0\.1)(:\d+)?$/
          def harborInsecure = env.REGISTRY_TYPE == 'Harbor' && (
            registryURL.startsWith('http://') ||
            !registryURL.startsWith('https://') ||
            privateRegistryHost
          )
          env.REGISTRY_INSECURE = harborInsecure ? 'true' : 'false'
          if (params.ROLLBACK_OF?.trim()) {
            echo "Rollback build of #${params.ROLLBACK_OF}"
          }
        }
        echo "SparkOps CI: ${env.APP_NAME} (${env.LANGUAGE}) in ${env.NAMESPACE}"
        echo "Git ${env.GIT_URL} @ ${env.BRANCH} (${env.GIT_REF_TYPE})"
        echo "Target image ${env.FULL_IMAGE}"
      }
    }

    stage('Checkout') {
      steps {
        script {
          def branch = (env.BRANCH ?: 'main').trim()
          def branchSpec = env.GIT_REF_TYPE == 'tag' ? "refs/tags/${branch}" : "*/${branch}"
          checkout([
            $class: 'GitSCM',
            branches: [[name: branchSpec]],
            extensions: [[$class: 'CloneOption', shallow: true, depth: 1]],
            userRemoteConfigs: [[url: env.GIT_URL, credentialsId: env.GIT_CREDENTIAL_ID]]
          ])
        }
      }
    }

    stage('Build') {
      steps {
        sh """
          set -e
          if ! command -v docker >/dev/null 2>&1; then
            echo "docker CLI not found on Jenkins agent"
            exit 1
          fi
          if ! docker info >/dev/null 2>&1; then
            echo "docker daemon not available (check /var/run/docker.sock mount)"
            exit 1
          fi
          docker build -f "\$DOCKERFILE_PATH" -t "${env.FULL_IMAGE}" .
        """
      }
    }
{{if eq .enable_static_scan "true"}}
    stage('Sonar Scan') {
      steps {
        echo "SonarQube static scan enabled for {{.name}}"
      }
    }
{{end}}
    stage('Push') {
      steps {
        script {
          withCredentials([usernamePassword(credentialsId: env.REGISTRY_CREDENTIAL_ID, usernameVariable: 'REGISTRY_USER', passwordVariable: 'REGISTRY_PASS')]) {
            sh """
              set -e
              REG_HOST="\$REGISTRY_HOST"
              if [ "${env.REGISTRY_INSECURE}" = "true" ] && command -v openssl >/dev/null 2>&1; then
                CERT_DIR="/etc/docker/certs.d/\${REG_HOST}"
                if mkdir -p "\$CERT_DIR" 2>/dev/null; then
                  if [ ! -s "\$CERT_DIR/ca.crt" ]; then
                    echo | openssl s_client -connect "\${REG_HOST}:443" -servername "\${REG_HOST}" 2>/dev/null \\
                      | sed -ne '/-BEGIN CERTIFICATE-/,/-END CERTIFICATE-/p' > "\$CERT_DIR/ca.crt" || true
                  fi
                fi
              fi
              echo "\$REGISTRY_PASS" | docker login "\$REG_HOST" -u "\$REGISTRY_USER" --password-stdin
              docker push "${env.FULL_IMAGE}"
            """
          }
        }
      }
    }

    stage('Complete') {
      steps {
        echo "Image ${env.FULL_IMAGE} is ready; SparkOps will deploy to namespace ${env.NAMESPACE}"
      }
    }
  }

  post {
    success {
      echo "Build succeeded: ${env.FULL_IMAGE}"
    }
    failure {
      echo 'Build failed'
    }
  }
}
`
}

func renderBuiltinJenkinsfile(vars map[string]string, enableScan bool) string {
	if enableScan {
		vars["enable_static_scan"] = "true"
	} else {
		vars["enable_static_scan"] = "false"
	}
	data := make(map[string]interface{}, len(vars))
	for k, v := range vars {
		data[k] = v
	}
	source := BuiltinJenkinsfileTemplateDetail()
	if out, err := renderGoTemplate(source, data); err == nil {
		return renderPlaceholders(out, vars)
	}
	return renderPlaceholders(source, vars)
}

func IsLegacyJenkinsfileTemplateDetail(detail string) bool {
	text := strings.TrimSpace(detail)
	if text == "" {
		return true
	}
	if !strings.Contains(text, builtinJenkinsfileVersionMarker) {
		return true
	}
	if strings.Contains(text, "Prepare Registry") {
		return true
	}
	if strings.Contains(text, "DOCKERFILE#./") {
		return true
	}
	if strings.Contains(text, "${env.DOCKERFILE_PATH}") {
		return true
	}
	if strings.Contains(text, "Kaniko") || strings.Contains(text, "kaniko") {
		return true
	}
	if !strings.Contains(text, "certs.d") {
		return true
	}
	return !strings.Contains(text, "REGISTRY_INSECURE")
}
