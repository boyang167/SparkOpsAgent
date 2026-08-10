import type { ChangePasswordCommand, UpdateProfileCommand } from "@sparkops/contracts/identity";
import type { MeVM } from "@sparkops/contracts/identity";
import { bffFetch } from "./client";

export function updateProfile(input: UpdateProfileCommand) {
  return bffFetch<MeVM>("/auth/me", {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function changePassword(input: ChangePasswordCommand) {
  return bffFetch<{ changed: true }>("/auth/password", {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}
