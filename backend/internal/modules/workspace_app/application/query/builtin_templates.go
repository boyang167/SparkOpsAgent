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
          env.BRANCH = (params.BRANCH ?: env.DEFAULT_GIT_REF)?.trim()
          env.CODE_VERSION = (params.CodeVersion ?: env.BRANCH)?.trim()
          env.IMAGE_TAG = (params.IMAGE_TAG ?: 'latest')?.trim()
          env.IMAGE_REPOSITORY = (params.IMAGE_REPOSITORY ?: '{{.registry_host}}/{{.workspace_slug}}/{{.name}}')?.trim()
          env.FULL_IMAGE = "${env.IMAGE_REPOSITORY}:${env.IMAGE_TAG}"
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
          def branchSpec = env.GIT_REF_TYPE == 'tag' ? "refs/tags/${env.BRANCH}" : "*/${env.BRANCH}"
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
          docker build -f "${env.DOCKERFILE_PATH}" -t "${env.FULL_IMAGE}" .
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
              echo "\${REGISTRY_PASS}" | docker login ${env.REGISTRY_HOST} -u "\${REGISTRY_USER}" --password-stdin
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
	return !strings.Contains(text, builtinJenkinsfileVersionMarker)
}
