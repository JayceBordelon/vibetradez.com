"use client";

import { AccountMenu } from "@/components/layout/account-menu";
import { useSession } from "@/lib/session";

export function LandingNavAccount() {
  const { user, loading } = useSession();
  if (loading || !user) return null;
  return <AccountMenu user={user} />;
}
