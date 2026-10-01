## Resources

Fathom registers two MCP resource types (via `resources/list` and `resources/read`).

| URI | MIME type | Contents |
|---|---|---|
| `fathom:///manifest` | `application/json` | Server manifest: registered tools, active profile, known service IDs, server version |
| `fathom:///service/<id>` | `application/json` | Per-service symbol summary: symbol count, file count, file list, kind breakdown |

### Reading the manifest

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "resources/read",
  "params": { "uri": "fathom:///manifest" }
}
```

Response `contents[0].text` is a JSON object:
```json
{
  "server": "fathom",
  "version": "1.0.0",
  "tools": ["query_symbols", "find_callers", ...],
  "active_profile": "coding",
  "services": ["auth-service", "payment-service"]
}
```

### Reading a service resource

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "resources/read",
  "params": { "uri": "fathom:///service/payment-service" }
}
```

Response `contents[0].text` is a JSON object:
```json
{
  "service": "payment-service",
  "symbol_count": 891,
  "file_count": 38,
  "files": ["internal/auth/token.go", ...],
  "kinds": {"GoFunc": 47, "GoType": 12, ...}
}
```
