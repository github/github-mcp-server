import { StrictMode, useState } from "react";
import type React from "react";
import { createRoot } from "react-dom/client";
import { Avatar, Text, Link, Heading, Spinner } from "@primer/react";
import {
  OrganizationIcon,
  LocationIcon,
  LinkIcon,
  MailIcon,
  PeopleIcon,
  RepoIcon,
  PersonIcon,
} from "@primer/octicons-react";
import { AppProvider } from "../../components/AppProvider";
import { useMcpApp } from "../../hooks/useMcpApp";

interface UserData {
  login: string;
  avatar_url?: string;
  details?: {
    name?: string;
    company?: string;
    location?: string;
    blog?: string;
    email?: string;
    twitter_username?: string;
    public_repos?: number;
    followers?: number;
    following?: number;
  };
}

function AvatarWithFallback({ src, login, size }: { src?: string; login: string; size: number }) {
  const [imgError, setImgError] = useState(false);
  
  if (!src || imgError) {
    return (
      <div
        style={{
          width: size,
          height: size,
          borderRadius: "50%",
          backgroundColor: "var(--bgColor-accent-muted)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          marginRight: 16,
          flexShrink: 0,
        }}
      >
        <PersonIcon size={size * 0.6} />
      </div>
    );
  }

  return (
    <Avatar
      src={src}
      size={size}
      onError={() => setImgError(true)}
      style={{ marginRight: 16 }}
    />
  );
}

function UserCard({
  user,
  onOpenLink,
}: {
  user: UserData;
  onOpenLink?: (url: string) => void;
}) {
  const d = user.details || {};
  const handleClick =
    onOpenLink &&
    ((url: string) => (e: React.MouseEvent) => {
      e.preventDefault();
      onOpenLink(url);
    });

  return (
    <div
      style={{
        borderWidth: 1,
        borderStyle: "solid",
        borderColor: "var(--borderColor-default)",
        borderRadius: 6,
        backgroundColor: "var(--bgColor-muted)",
        padding: 16,
        maxWidth: 400,
      }}
    >
      {/* Header with avatar and name */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          marginBottom: 16,
          paddingBottom: 16,
          borderBottomWidth: 1,
          borderBottomStyle: "solid",
          borderBottomColor: "var(--borderColor-default)",
        }}
      >
        <AvatarWithFallback src={user.avatar_url} login={user.login} size={48} />
        <div>
          <Heading as="h2" style={{ fontSize: 16, marginBottom: 0 }}>
            {d.name || user.login}
          </Heading>
          <Text style={{ color: "var(--fgColor-muted)", fontSize: 14 }}>@{user.login}</Text>
        </div>
      </div>

      {/* Info grid */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "auto 1fr",
          gap: 8,
          fontSize: 14,
        }}
      >
        {d.company && (
          <>
            <div style={{ color: "var(--fgColor-muted)" }}><OrganizationIcon size={16} /></div>
            <Text>{d.company}</Text>
          </>
        )}
        {d.location && (
          <>
            <div style={{ color: "var(--fgColor-muted)" }}><LocationIcon size={16} /></div>
            <Text>{d.location}</Text>
          </>
        )}
        {d.blog && (
          <>
            <div style={{ color: "var(--fgColor-muted)" }}><LinkIcon size={16} /></div>
            <Link
              href={d.blog}
              target="_blank"
              onClick={handleClick?.(d.blog)}
            >
              {d.blog}
            </Link>
          </>
        )}
        {d.email && (
          <>
            <div style={{ color: "var(--fgColor-muted)" }}><MailIcon size={16} /></div>
            <Link href={`mailto:${d.email}`}>{d.email}</Link>
          </>
        )}
      </div>

      {/* Stats */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-around",
          marginTop: 16,
          paddingTop: 16,
          borderTopWidth: 1,
          borderTopStyle: "solid",
          borderTopColor: "var(--borderColor-default)",
        }}
      >
        <div style={{ textAlign: "center" }}>
          <Text style={{ fontWeight: 600, fontSize: 16, display: "block" }}>
            <RepoIcon size={16} /> {d.public_repos ?? 0}
          </Text>
          <Text style={{ color: "var(--fgColor-muted)", fontSize: 12 }}>Repos</Text>
        </div>
        <div style={{ textAlign: "center" }}>
          <Text style={{ fontWeight: 600, fontSize: 16, display: "block" }}>
            <PeopleIcon size={16} /> {d.followers ?? 0}
          </Text>
          <Text style={{ color: "var(--fgColor-muted)", fontSize: 12 }}>Followers</Text>
        </div>
        <div style={{ textAlign: "center" }}>
          <Text style={{ fontWeight: 600, fontSize: 16, display: "block" }}>
            {d.following ?? 0}
          </Text>
          <Text style={{ color: "var(--fgColor-muted)", fontSize: 12 }}>Following</Text>
        </div>
      </div>
    </div>
  );
}

function GetMeApp() {
  const { error, toolResult, hostContext, openLink } = useMcpApp({
    appName: "github-mcp-server-get-me",
  });

  const content = (() => {
    if (error) {
      return <Text style={{ color: "var(--fgColor-danger)" }}>Error: {error.message}</Text>;
    }
    if (!toolResult) {
      return (
        <div style={{ display: "flex", alignItems: "center", gap: 2 }}>
          <Spinner size="small" />
          <Text style={{ color: "var(--fgColor-muted)" }}>Loading user data...</Text>
        </div>
      );
    }
    const textContent = toolResult.content?.find((c: { type: string }) => c.type === "text");
    if (!textContent || !("text" in textContent)) {
      return <Text style={{ color: "var(--fgColor-danger)" }}>No user data in response</Text>;
    }
    try {
      const userData = JSON.parse(textContent.text as string) as UserData;
      return <UserCard user={userData} onOpenLink={(url) => void openLink(url)} />;
    } catch {
      return <Text style={{ color: "var(--fgColor-danger)" }}>Failed to parse user data</Text>;
    }
  })();

  return <AppProvider hostContext={hostContext}>{content}</AppProvider>;
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <GetMeApp />
  </StrictMode>
);
