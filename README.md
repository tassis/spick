# spick

`spick` manages project manifests and resource manifests.

- `spick init` scaffolds a project or resource manifest.
- `spick add <source>` adds from a source with `--skill`, `--plugin`, or `--agent` narrowing.
- `spick rm` removes from managed state with the same narrowing flags.
- `spick apply` edits desired state at the root level, then hands off to `sync`.
- `spick apply --runtime <id>` targets a runtime; use `--skill`, `--plugin`, or `--agent` to narrow a class.
- `spick sync -g/--global` reconciles global state; project sync is the default.
- `spick inspect <source>` inspects a source.
- `spick list` shows skills, plugins, and agents by default; use `--skill` or `--plugins` to narrow it.

Resource manifests use `spick.res.yaml` with `kind: plugin` or `kind: resources`.
Plugin repos without `spick.res.yaml` are not inferred.

Project manifests keep declared resources in `project.skills`, `project.plugins`, and `project.agents`.
Runtime desired state lives under `project.runtimes`.
