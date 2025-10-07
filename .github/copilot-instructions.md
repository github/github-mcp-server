## Objectif

Fournir à un agent IA les informations essentielles pour être immédiatement productif dans ce dépôt : architecture, points d'entrée, workflows de build/test/exécution, conventions propres au projet et exemples concrets tirés du code.

## Big picture (où regarder en premier)

- Commandes / points d'entrée : `cmd/github-mcp-server/main.go` (binaire principal) et `cmd/mcpcurl/` (outil utilitaire).
- Serveur MCP : `internal/ghmcp/server.go` — assemble les clients REST/GraphQL, crée les toolsets et démarre le serveur stdio.
- Logic métier et toolsets : `pkg/github/` — définit les outils, helper patterns, pagination et generation d'instructions.
- Clients bas niveau : `pkg/raw/`, `pkg/translations/`, `pkg/log/` — utilitaires pour accès raw, i18n et logging.

Lire ces fichiers dans l'ordre ci‑dessous pour comprendre le flux : `main.go` → `internal/ghmcp/server.go` → `pkg/github/*.go`.

## Commandes essentielles (exemples)

- Build local :

```bash
cd cmd/github-mcp-server
go build -o github-mcp-server
```

- Démarrer en mode stdio (nécessite `GITHUB_PERSONAL_ACCESS_TOKEN`):

```bash
GITHUB_PERSONAL_ACCESS_TOKEN=<token> ./github-mcp-server stdio
```

- Docker (image publique) :

```bash
docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN=<token> ghcr.io/github/github-mcp-server
```

- Tests E2E (nécessitent jeton et Docker) :

```bash
GITHUB_MCP_SERVER_E2E_TOKEN=<YOUR TOKEN> go test -v --tags e2e ./e2e
```

## Flags et variables d'environnement importantes

- `GITHUB_PERSONAL_ACCESS_TOKEN` : token utilisé pour les appels GitHub (voir `main.go` / viper).
- Flags globaux exposés par le binaire (`main.go`): `--toolsets`, `--dynamic-toolsets`, `--read-only`, `--log-file`, `--enable-command-logging`, `--export-translations`, `--content-window-size`.
- `GITHUB_TOOLSETS` peut aussi être utilisé pour définir les toolsets via variable d'env.

## Conventions et patterns spécifiques

- Configuration : utilisation de `spf13/cobra` + `spf13/viper`. Les noms de flags utilisent `-` (normalisation dans `main.go`).
- Auth et clients : REST via `github.com/google/go-github`, GraphQL via `github.com/shurcooL/githubv4`. Voir `internal/ghmcp.NewMCPServer`.
- Tool definitions : utilisez `pkg/github` helpers (ex. `WithPagination`, `WithUnifiedPagination`, `OptionalParam`, `RequiredParam`) pour définir paramètres et pagination uniformes.
- Erreurs GitHub : le contexte transporte des erreurs spécifiques (voir `pkg/errors` et `isAcceptedError` dans `pkg/github`).
- Traductions : i18n est exposé via `pkg/translations` et peut être exporté avec `--export-translations`.

## Intégrations externes et points de vigilance

- Dépendances clés : `mark3labs/mcp-go` (MCP primitives), `google/go-github` (REST), `shurcooL/githubv4` (GraphQL). Modifier la façon dont clients sont construits peut impacter tous les toolsets.
- API host parsing : `internal/ghmcp/parseAPIHost` gère `github.com`, GHEC et GHES. Pour tests locaux attention aux ports et schemes.
- Dynamic toolsets : si activé (`--dynamic-toolsets`) le code filtre `all` et enregistre dynamiquement des outils (voir `pkg/github` flows dans `internal/ghmcp`).

## Exemples concrets tirés du code

- Récupérer le token et démarrer le serveur stdio (de `main.go`): le serveur réclame `GITHUB_PERSONAL_ACCESS_TOKEN` et construit `StdioServerConfig`.
- Pagination uniforme : `pkg/github.WithUnifiedPagination()` ajoute `page`, `perPage` et `after` — GraphQL convertira ces paramètres en curseurs.
- Helpers paramètres : `RequiredParam[T]`, `OptionalParam[T]` standardisent la validation des arguments d'outils.

## Où poser des modifications sûres

- Ajouter ou modifier un toolset : travailler dans `pkg/github/` et exécuter les tests unitaires. Les changements d'API publique nécessitent mise à jour des instructions auto-générées (`github.GenerateInstructions`).
- Changer la manière dont les clients sont construits : `internal/ghmcp.NewMCPServer` — impact global, testez manuellement avec `stdio` et via E2E.

## Raccourci pour l'agent

- Pour explorer le comportement d'un outil : lire le fichier `pkg/github/<tool>_*.go` correspondant, repérer les `mcp.With...` paramètres et tester l'appel via un client stdio (initialisation + CallTool JSON-RPC).

Si tu veux, j'intègre des extraits d'exemples JSON-RPC d'initialisation / d'appel d'outils et j'ajoute une section « checklist de PR » pour agents (ex. build, lint, tests unitaires, e2e flag). Veux-tu que je l'ajoute ?
