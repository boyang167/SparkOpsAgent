export function hasStorageAdmin(permissions: string[]) {
  return permissions.includes("storage:admin");
}

export function hasLifecycleAdmin(permissions: string[]) {
  return permissions.includes("lifecycle:admin");
}

export function canMutateStorage(permissions: string[]) {
  return hasStorageAdmin(permissions);
}

export function canMutateLifecycle(permissions: string[]) {
  return hasLifecycleAdmin(permissions);
}
