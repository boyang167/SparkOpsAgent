package kernel

import "github.com/google/uuid"

type MemberID struct{ uuid.UUID }
type OrgID struct{ uuid.UUID }
type WorkspaceID struct{ uuid.UUID }
type ProjectID struct{ uuid.UUID }
type AgentID struct{ uuid.UUID }
type AgentVersionID struct{ uuid.UUID }
type RoleID struct{ uuid.UUID }
type RoleBindingID struct{ uuid.UUID }
type ConversationID struct{ uuid.UUID }
type RunID struct{ uuid.UUID }
type MessageID struct{ uuid.UUID }
type WorkflowID struct{ uuid.UUID }
type WorkflowRunID struct{ uuid.UUID }
type KnowledgeBaseID struct{ uuid.UUID }
type DocumentID struct{ uuid.UUID }
type NotificationID struct{ uuid.UUID }
type BuildID struct{ uuid.UUID }
type DeploymentRunID struct{ uuid.UUID }
type CronJobID struct{ uuid.UUID }
type PodID struct{ uuid.UUID }
type TemplateID struct{ uuid.UUID }
type SecretID struct{ uuid.UUID }
type ProductID struct{ uuid.UUID }
type ProductPlanID struct{ uuid.UUID }
type ProductRequirementID struct{ uuid.UUID }
type ClusterInfoID struct{ uuid.UUID }
type ProclaimID struct{ uuid.UUID }

func NewMemberID() MemberID     { return MemberID{UUID: uuid.New()} }
func NewOrgID() OrgID             { return OrgID{UUID: uuid.New()} }
func ParseOrgID(s string) (OrgID, error) {
	id, err := uuid.Parse(s)
	return OrgID{UUID: id}, err
}
func (id OrgID) String() string { return id.UUID.String() }
func NewWorkspaceID() WorkspaceID { return WorkspaceID{UUID: uuid.New()} }
func NewProjectID() ProjectID   { return ProjectID{UUID: uuid.New()} }
func NewAgentID() AgentID                 { return AgentID{UUID: uuid.New()} }
func NewAgentVersionID() AgentVersionID   { return AgentVersionID{UUID: uuid.New()} }
func NewRoleID() RoleID         { return RoleID{UUID: uuid.New()} }
func NewRoleBindingID() RoleBindingID { return RoleBindingID{UUID: uuid.New()} }
func NewConversationID() ConversationID { return ConversationID{UUID: uuid.New()} }
func NewRunID() RunID                   { return RunID{UUID: uuid.New()} }
func NewMessageID() MessageID           { return MessageID{UUID: uuid.New()} }
func NewWorkflowID() WorkflowID           { return WorkflowID{UUID: uuid.New()} }
func NewWorkflowRunID() WorkflowRunID     { return WorkflowRunID{UUID: uuid.New()} }
func NewKnowledgeBaseID() KnowledgeBaseID { return KnowledgeBaseID{UUID: uuid.New()} }
func NewDocumentID() DocumentID           { return DocumentID{UUID: uuid.New()} }
func NewNotificationID() NotificationID     { return NotificationID{UUID: uuid.New()} }
func NewBuildID() BuildID                     { return BuildID{UUID: uuid.New()} }
func NewDeploymentRunID() DeploymentRunID     { return DeploymentRunID{UUID: uuid.New()} }
func NewCronJobID() CronJobID                 { return CronJobID{UUID: uuid.New()} }
func NewPodID() PodID                         { return PodID{UUID: uuid.New()} }
func NewTemplateID() TemplateID               { return TemplateID{UUID: uuid.New()} }
func NewSecretID() SecretID                   { return SecretID{UUID: uuid.New()} }
func NewProductID() ProductID                 { return ProductID{UUID: uuid.New()} }
func NewProductPlanID() ProductPlanID         { return ProductPlanID{UUID: uuid.New()} }
func NewProductRequirementID() ProductRequirementID {
	return ProductRequirementID{UUID: uuid.New()}
}
func NewClusterInfoID() ClusterInfoID         { return ClusterInfoID{UUID: uuid.New()} }
func NewProclaimID() ProclaimID               { return ProclaimID{UUID: uuid.New()} }

func ParseRoleID(s string) (RoleID, error) {
	id, err := uuid.Parse(s)
	return RoleID{UUID: id}, err
}

func ParseMemberID(s string) (MemberID, error) {
	id, err := uuid.Parse(s)
	return MemberID{UUID: id}, err
}

func ParseWorkspaceID(s string) (WorkspaceID, error) {
	id, err := uuid.Parse(s)
	return WorkspaceID{UUID: id}, err
}

func ParseProjectID(s string) (ProjectID, error) {
	id, err := uuid.Parse(s)
	return ProjectID{UUID: id}, err
}

func ParseAgentID(s string) (AgentID, error) {
	id, err := uuid.Parse(s)
	return AgentID{UUID: id}, err
}

func ParseAgentVersionID(s string) (AgentVersionID, error) {
	id, err := uuid.Parse(s)
	return AgentVersionID{UUID: id}, err
}

func ParseConversationID(s string) (ConversationID, error) {
	id, err := uuid.Parse(s)
	return ConversationID{UUID: id}, err
}

func ParseRunID(s string) (RunID, error) {
	id, err := uuid.Parse(s)
	return RunID{UUID: id}, err
}

func ParseMessageID(s string) (MessageID, error) {
	id, err := uuid.Parse(s)
	return MessageID{UUID: id}, err
}

func ParseWorkflowID(s string) (WorkflowID, error) {
	id, err := uuid.Parse(s)
	return WorkflowID{UUID: id}, err
}

func ParseWorkflowRunID(s string) (WorkflowRunID, error) {
	id, err := uuid.Parse(s)
	return WorkflowRunID{UUID: id}, err
}

func ParseKnowledgeBaseID(s string) (KnowledgeBaseID, error) {
	id, err := uuid.Parse(s)
	return KnowledgeBaseID{UUID: id}, err
}

func ParseDocumentID(s string) (DocumentID, error) {
	id, err := uuid.Parse(s)
	return DocumentID{UUID: id}, err
}

func ParseNotificationID(s string) (NotificationID, error) {
	id, err := uuid.Parse(s)
	return NotificationID{UUID: id}, err
}

func ParseBuildID(s string) (BuildID, error) {
	id, err := uuid.Parse(s)
	return BuildID{UUID: id}, err
}

func ParseDeploymentRunID(s string) (DeploymentRunID, error) {
	id, err := uuid.Parse(s)
	return DeploymentRunID{UUID: id}, err
}

func ParseCronJobID(s string) (CronJobID, error) {
	id, err := uuid.Parse(s)
	return CronJobID{UUID: id}, err
}

func ParsePodID(s string) (PodID, error) {
	id, err := uuid.Parse(s)
	return PodID{UUID: id}, err
}

func ParseTemplateID(s string) (TemplateID, error) {
	id, err := uuid.Parse(s)
	return TemplateID{UUID: id}, err
}

func ParseSecretID(s string) (SecretID, error) {
	id, err := uuid.Parse(s)
	return SecretID{UUID: id}, err
}

func ParseProductID(s string) (ProductID, error) {
	id, err := uuid.Parse(s)
	return ProductID{UUID: id}, err
}

func ParseProductPlanID(s string) (ProductPlanID, error) {
	id, err := uuid.Parse(s)
	return ProductPlanID{UUID: id}, err
}

func ParseProductRequirementID(s string) (ProductRequirementID, error) {
	id, err := uuid.Parse(s)
	return ProductRequirementID{UUID: id}, err
}

func ParseClusterInfoID(s string) (ClusterInfoID, error) {
	id, err := uuid.Parse(s)
	return ClusterInfoID{UUID: id}, err
}

func ParseProclaimID(s string) (ProclaimID, error) {
	id, err := uuid.Parse(s)
	return ProclaimID{UUID: id}, err
}

func (id MemberID) String() string        { return id.UUID.String() }
func (id WorkspaceID) String() string     { return id.UUID.String() }
func (id ProjectID) String() string       { return id.UUID.String() }
func (id AgentID) String() string              { return id.UUID.String() }
func (id AgentVersionID) String() string       { return id.UUID.String() }
func (id ConversationID) String() string       { return id.UUID.String() }
func (id RunID) String() string                 { return id.UUID.String() }
func (id MessageID) String() string             { return id.UUID.String() }
func (id RoleID) String() string          { return id.UUID.String() }
func (id RoleBindingID) String() string   { return id.UUID.String() }
func (id WorkflowID) String() string       { return id.UUID.String() }
func (id WorkflowRunID) String() string    { return id.UUID.String() }
func (id KnowledgeBaseID) String() string  { return id.UUID.String() }
func (id DocumentID) String() string       { return id.UUID.String() }
func (id NotificationID) String() string    { return id.UUID.String() }
func (id BuildID) String() string            { return id.UUID.String() }
func (id DeploymentRunID) String() string    { return id.UUID.String() }
func (id CronJobID) String() string          { return id.UUID.String() }
func (id PodID) String() string               { return id.UUID.String() }
func (id TemplateID) String() string          { return id.UUID.String() }
func (id SecretID) String() string            { return id.UUID.String() }
func (id ProductID) String() string           { return id.UUID.String() }
func (id ProductPlanID) String() string       { return id.UUID.String() }
func (id ProductRequirementID) String() string { return id.UUID.String() }
func (id ClusterInfoID) String() string       { return id.UUID.String() }
func (id ProclaimID) String() string          { return id.UUID.String() }
