"use server";

import { revalidatePath } from "next/cache";
import { cookies } from "next/headers";

export async function revalidateRegistryCache() {
  const cookieStore = await cookies();
  const token = cookieStore.get("sb-access-token");
  
  if (!token) {
    throw new Error("Unauthorized to perform revalidation.");
  }
  
  revalidatePath("/registry");
  revalidatePath("/users/[username]", "page");
  revalidatePath("/", "layout");
}
