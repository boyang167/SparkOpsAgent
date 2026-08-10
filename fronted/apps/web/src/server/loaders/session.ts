import { cookies } from "next/headers";
import { createBffContextFromToken } from "@sparkops/bff-core";
import { getSession } from "@sparkops/bff-modules/identity";
import type { MeVM } from "@sparkops/contracts/identity";

export async function loadSession(): Promise<MeVM | null> {
  const cookieStore = await cookies();
  const token = cookieStore.get("sparkops_token")?.value;
  if (!token) {
    return null;
  }
  try {
    const ctx = createBffContextFromToken(token);
    return await getSession(ctx);
  } catch {
    return null;
  }
}
