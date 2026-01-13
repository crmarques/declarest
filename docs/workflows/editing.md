# Editing contexts & metadata

DeclaREST helps you keep repository metadata and context definitions in sync by opening templates in your preferred editor, so you can edit defaults, metadata rules, or entire contexts without hand‑crafting JSON/YAML.

## Edit context configurations

Use `declarest config edit <name>` (or `--name <name>`) to open the stored context in your editor. DeclaREST pre-fills the file with the default configuration schema and your current values so you can tweak repository/auth settings, TLS options, or secret store credentials:

```
declarest config edit staging --editor "code --wait"
```

DeclaREST pre-fills the file with every attribute (using defaults for anything you have not defined) and removes those defaults before saving so the stored file stays clean. Inline comments now describe each field, and a header line reminds you defaults are stripped and that the CLI ignores the guidance comments before parsing. Saving without editing anything leaves the stored config untouched and prints `[OK] no updates detected`. If the named context does not exist yet, the command creates it once you save the file. If the argument you're passing points to an existing context YAML (for example `contexts/staging.yaml`), DeclaREST loads that file so you see the attributes you already wrote. The `--editor` flag overrides the default `vi`.

Before writing the context back to the store, DeclaREST parses and validates your edits. Syntax errors, schema violations, or unsupported combinations (like specifying both git and filesystem repositories or multiple secret store auth methods) will cause the command to abort; fix the issues and save the file again.

## Edit metadata rules

Use `declarest metadata edit <path>` to open the merged metadata template for a collection or resource. The CLI preloads metadata defaults (IDs, operations, headers, filters, etc.) so every attribute is present, and, when you save, strips those defaults again so only your overrides remain in the local metadata file. Inline comments annotate every attribute and a header note reminds you that the CLI ignores the comment lines before parsing.

```
declarest metadata edit /teams/platform/users/
```

By default the command treats paths without a trailing `/` as collections; add `--for-resource-only` when you need to target the single resource metadata instead:

```
declarest metadata edit /teams/platform/users/alice --for-resource-only
```

You can also override the editor with `--editor` (just like `config edit`). The default editor is `vi`. Save the file when you are done, and DeclaREST writes the changes to the correct `<collection>/_/metadata.json` or `<resource>/metadata.json`.

The metadata editor also validates the output before writing: invalid JSON or incompatible field types (for example, `secretInAttributes` must be an array) will stop the save so you can correct the payload.
