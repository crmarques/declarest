# Agents

## Completion behavior
- Static OpenAPI path segments are suggested before querying repo/remote origin data.
- When the input ends with `/` for a collection, completions list items as `<collection>/<id> (alias)`, omitting the alias when `idFromAttribute` equals `aliasFromAttribute`.
