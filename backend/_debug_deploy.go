package main
import (
  "fmt"
  "log"
  "github.com/sparkops/sparkops-agent/backend/internal/platform/config"
  "github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)
func main() {
  cfg, _ := config.LoadFromEnv()
  db, err := postgres.Open(cfg.Database.DSN)
  if err != nil { log.Fatal(err) }
  type run struct { ID, Status, ErrorMessage, GitRef string; Attempt int }
  var runs []run
  db.Raw(`SELECT id, status, error_message, git_ref_value, attempt_count FROM delivery.deployment_runs ORDER BY created_at DESC LIMIT 3`).Scan(&runs)
  for _, r := range runs { fmt.Printf("%+v\n", r) }
  type proj struct { ID, Name, NS, GitURL, Review, JenkinsID, SecretID, Harbor string; Resources []byte }
  var p proj
  db.Raw(`SELECT id, name, namespace, git_url, review_status, jenkins_secret_id, secret_id, resources FROM workspace_app.projects WHERE id = 'acf32f29-8373-4898-b570-59c92ae9c600'`).Scan(&p)
  fmt.Printf("project=%+v\n", p)
  type secret struct { ID, Type, URL, User string }
  var secrets []secret
  db.Raw(`SELECT id, service_type, service_url, username FROM workspace_app.secrets WHERE workspace_id = 'bc55a5d8-6ba0-47e2-887a-81ed7f57aa8e'`).Scan(&secrets)
  for _, s := range secrets { fmt.Printf("secret=%+v\n", s) }
}
