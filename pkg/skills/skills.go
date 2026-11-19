// Package skills implements the server half of the MCP Skills extension:
// https://github.com/modelcontextprotocol/ext-skills/blob/main/specification/beta/skills.mdx.
//
// Skills are directories of ordinary resources, entered through a SKILL.md.
// Registry holds validated files and manifests; Publisher serves skills/list,
// skills/get, resources/read and optional directory reads using the MCP SDK.
// The bundled GitHub workflow catalog lives in the catalog subpackage.
package skills

const (
	// ExtensionID is the capability extension identifier.
	ExtensionID = "io.modelcontextprotocol/skills"
	// SettingDirectoryRead advertises resources/directory/read support.
	SettingDirectoryRead = "directoryRead"
	// URIScheme is the scheme used for skill resources.
	URIScheme = "skill"
	// SkillFile is the required entry file of every skill.
	SkillFile = "SKILL.md"
	// DirectoryMIMEType identifies directories in resource listings.
	DirectoryMIMEType = "inode/directory"
	// SkillMIMEType is the media type of SKILL.md.
	SkillMIMEType = "text/markdown"
	// MetaPrefix is reserved for extension-defined resource metadata.
	MetaPrefix = "io.modelcontextprotocol.skills/"
	// ReservedMetadataPrefix is reserved within frontmatter metadata.
	ReservedMetadataPrefix = "io.modelcontextprotocol/"
	// MaxResourcesPerSkill includes SKILL.md and every supporting file.
	MaxResourcesPerSkill = 512
	// MaxSkillBytes is the sum of all raw file sizes in a skill.
	MaxSkillBytes int64 = 16 * 1024 * 1024
)
