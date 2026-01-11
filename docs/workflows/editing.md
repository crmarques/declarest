# Editing contexts & metadata

DeclaREST helps you keep repository metadata and context definitions in sync by opening templates in your preferred editor, so you can edit defaults, metadata rules, or entire contexts without hand‑crafting JSON/YAML.

## Edit context configurations

Use `declarest config edit <name>` (or `--name <name>`) to open the stored context in your editor. DeclaREST pre-fills the file with the default configuration schema and your current values so you can tweak repository/auth settings, TLS options, or secret store credentials:

```
declarest config edit staging --editor "code --wait"
```

If the named context does not exist yet, the command creates it once you save the file. The `--editor` flag overrides `$VISUAL`/`$EDITOR` (defaults to `vi` when neither is set).

## Edit metadata rules

Use `declarest metadata edit <path>` to open the merged metadata template for a collection or resource. The CLI preloads metadata defaults (IDs, operations, headers, filters, etc.) and, when you save, strips the default values so only your overrides remain in the local metadata file:

```
declarest metadata edit /teams/platform/users/
```

By default the command treats paths without a trailing `/` as collections; add `--for-resource-only` when you need to target the single resource metadata instead:

```
declarest metadata edit /teams/platform/users/alice --for-resource-only
```

You can also override the editor with `--editor` (just like `config edit`). Save the file when you are done, and DeclaREST writes the changes to the correct `<collection>/_/metadata.json` or `<resource>/metadata.json`.
