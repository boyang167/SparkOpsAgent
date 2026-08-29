export interface WorkspaceRequestIdentity {
  workspaceId: string;
  generation: number;
}

export interface WorkspaceAuthorizationState<T extends { id: string }>
  extends WorkspaceRequestIdentity {
  items: T[];
  error: string | null;
  loading?: boolean;
}

export function selectWorkspaceAuthorization<T extends { id: string }>(
  state: WorkspaceAuthorizationState<T>,
  workspaceId: string,
): WorkspaceAuthorizationState<T> {
  return {
    workspaceId,
    generation: state.generation + 1,
    items: [],
    error: null,
    loading: Boolean(workspaceId),
  };
}

export function isCurrentWorkspaceRequest(
  current: WorkspaceRequestIdentity,
  request: WorkspaceRequestIdentity,
) {
  return (
    current.workspaceId === request.workspaceId &&
    current.generation === request.generation
  );
}
