package deps

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"path"
)

// A repository inventory belongs to exactly one ReadFile or ReadFiles call.
// Reader options and the caller's FS identity never retain this evidence.
// Batches are read sequentially, so the lazy inventory needs no shared lock.
type tsjsPackageInventory struct {
	ready     bool
	names     map[string]bool
	manifests []string
	code      UnknownCode
}

func (g *tsjsRead) provePackageAbsent(pkg string) UnknownCode {
	inventory := g.repo.tsPackages
	if inventory == nil {
		return UnknownMissingConfig
	}
	if !inventory.ready {
		inventory.names = make(map[string]bool)
		inventory.code = inventory.load(g.ctx, g.repo)
		inventory.ready = true
	}
	// These manifests are inputs to the absence proof even when their names
	// differ from the operand. Expose the consulted files as config edges.
	if !g.localityInputs {
		for _, file := range inventory.manifests {
			g.edge(KindConfigRef, file, file, ResolvedFile, "", 0)
		}
		g.localityInputs = true
	}
	if inventory.code != "" {
		return inventory.code
	}
	if inventory.names[pkg] {
		// Finding a package identity does not establish a supported target.
		// Alias, self-export and visible package resolution already ran.
		return UnknownUnresolvedAlias
	}
	return ""
}

func (i *tsjsPackageInventory) load(ctx context.Context, repo repositoryFS) UnknownCode {
	// Bound both traversal and manifest bytes. Reaching a bound cannot prove
	// absence and therefore withholds completeness under the existing sets.
	const maxInventoryEntries = 32 * maxPackageEntries
	const maxInventoryBytes = 32 * maxReadBytes
	dirs := []string{"."}
	entriesSeen, bytesRead := 0, 0
	for len(dirs) != 0 {
		if ctx.Err() != nil {
			return UnknownMissingConfig
		}
		dir := dirs[len(dirs)-1]
		dirs = dirs[:len(dirs)-1]
		entries, code := packageEntries(repo, dir)
		if code != "" {
			return code
		}
		entriesSeen += len(entries)
		if entriesSeen > maxInventoryEntries {
			return UnknownReadLimit
		}
		for _, entry := range entries {
			if ctx.Err() != nil {
				return UnknownMissingConfig
			}
			info, err := entry.Info()
			if err != nil {
				return UnknownMissingConfig
			}
			name := path.Join(dir, entry.Name())
			switch info.Mode().Type() {
			case fs.ModeDir:
				if !validPath(name) {
					return UnknownUnsupportedConstruct
				}
				dirs = append(dirs, name)
			case 0:
				if entry.Name() != "package.json" {
					continue
				}
				data, code := readSource(repo, name)
				if code != "" {
					return code
				}
				i.manifests = append(i.manifests, name)
				bytesRead += len(data)
				if bytesRead > maxInventoryBytes || len(i.manifests) > maxPackageEntries {
					return UnknownReadLimit
				}
				if !json.Valid(data) {
					return UnknownParseError
				}
				properties, ok := tsjsObject(data)
				if !ok {
					return UnknownParseError
				}
				for _, property := range properties {
					if property.key != "name" {
						continue
					}
					var identity string
					if !bytes.HasPrefix(bytes.TrimSpace(property.value), []byte{'"'}) || json.Unmarshal(property.value, &identity) != nil {
						return UnknownParseError
					}
					if identity != "" {
						i.names[identity] = true
					}
				}
			default:
				// A symlink could conceal a directory. Never follow it, or
				// claim absence after ignoring an uninspectable region.
				return UnknownUnsupportedConstruct
			}
		}
	}
	return ""
}
