import { StrictMode, useState } from "react";
import { createRoot } from "react-dom/client";
import { Avatar, Box, Text, Link, Heading, Spinner } from "@primer/react";
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
      <Box
        sx={{
          width: size,
          height: size,
          borderRadius: "50%",
          bg: "accent.subtle",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          mr: 3,
          flexShrink: 0,
        }}
      >
        <PersonIcon size={size * 0.6} />
      </Box>
    );
  }

  return (
    <Avatar 
      src={src} 
      size={size} 
      sx={{ mr: 3 }} 
      onError={() => setImgError(true)}
    />
  );
}

function UserCard({ user }: { user: UserData }) {
  const d = user.details || {};

  return (
    <Box
      borderWidth={1}
      borderStyle="solid"
      borderColor="border.default"
      borderRadius={2}
      bg="canvas.subtle"
      p={3}
      maxWidth={400}
    >
      {/* Header with avatar and name */}
      <Box display="flex" alignItems="center" mb={3} pb={3} borderBottomWidth={1} borderBottomStyle="solid" borderBottomColor="border.default">
        <AvatarWithFallback src={user.avatar_url} login={user.login} size={48} />
        <Box>
          <Heading as="h2" sx={{ fontSize: 2, mb: 0 }}>
            {d.name || user.login}
          </Heading>
          <Text sx={{ color: "fg.muted", fontSize: 1 }}>@{user.login}</Text>
        </Box>
      </Box>

      {/* Info grid */}
      <Box display="grid" sx={{ gridTemplateColumns: "auto 1fr", gap: 2, fontSize: 1 }}>
        {d.company && (
          <>
            <Box sx={{ color: "fg.muted" }}><OrganizationIcon size={16} /></Box>
            <Text>{d.company}</Text>
          </>
        )}
        {d.location && (
          <>
            <Box sx={{ color: "fg.muted" }}><LocationIcon size={16} /></Box>
            <Text>{d.location}</Text>
          </>
        )}
        {d.blog && (
          <>
            <Box sx={{ color: "fg.muted" }}><LinkIcon size={16} /></Box>
            <Link href={d.blog} target="_blank">{d.blog}</Link>
          </>
        )}
        {d.email && (
          <>
            <Box sx={{ color: "fg.muted" }}><MailIcon size={16} /></Box>
            <Link href={`mailto:${d.email}`}>{d.email}</Link>
          </>
        )}
      </Box>

      {/* Stats */}
      <Box display="flex" justifyContent="space-around" mt={3} pt={3} borderTopWidth={1} borderTopStyle="solid" borderTopColor="border.default">
        <Box sx={{ textAlign: "center" }}>
          <Text sx={{ fontWeight: "bold", fontSize: 2, display: "block" }}>
            <RepoIcon size={16} /> {d.public_repos ?? 0}
          </Text>
          <Text sx={{ color: "fg.muted", fontSize: 0 }}>Repos</Text>
        </Box>
        <Box sx={{ textAlign: "center" }}>
          <Text sx={{ fontWeight: "bold", fontSize: 2, display: "block" }}>
            <PeopleIcon size={16} /> {d.followers ?? 0}
          </Text>
          <Text sx={{ color: "fg.muted", fontSize: 0 }}>Followers</Text>
        </Box>
        <Box sx={{ textAlign: "center" }}>
          <Text sx={{ fontWeight: "bold", fontSize: 2, display: "block" }}>
            {d.following ?? 0}
          </Text>
          <Text sx={{ color: "fg.muted", fontSize: 0 }}>Following</Text>
        </Box>
      </Box>
    </Box>
  );
}

function GetMeApp() {
  const { error, toolResult } = useMcpApp({
    appName: "github-mcp-server-get-me",
  });

  if (error) {
    return <Text sx={{ color: "danger.fg" }}>Error: {error.message}</Text>;
  }

  if (!toolResult) {
    return (
      <Box display="flex" alignItems="center" gap={2}>
        <Spinner size="small" />
        <Text sx={{ color: "fg.muted" }}>Loading user data...</Text>
      </Box>
    );
  }

  // Parse user data from tool result
  const textContent = toolResult.content?.find((c: { type: string }) => c.type === "text");
  if (!textContent || !("text" in textContent)) {
    return <Text sx={{ color: "danger.fg" }}>No user data in response</Text>;
  }

  try {
    const userData = JSON.parse(textContent.text as string) as UserData;
    return <UserCard user={userData} />;
  } catch {
    return <Text sx={{ color: "danger.fg" }}>Failed to parse user data</Text>;
  }
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <AppProvider>
      <GetMeApp />
    </AppProvider>
  </StrictMode>
);
