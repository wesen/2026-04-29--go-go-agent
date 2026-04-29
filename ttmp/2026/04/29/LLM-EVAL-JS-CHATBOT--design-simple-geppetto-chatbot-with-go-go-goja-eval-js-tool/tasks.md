# Tasks

## Done

- [x] Create docmgr ticket workspace.
- [x] Add primary design document.
- [x] Add investigation diary.
- [x] Create ticket-local evidence/inventory script.
- [x] Generate source evidence snippets and doc inventories.
- [x] Write intern-facing design and implementation guide.
- [x] Include relevant documentation starting points grouped by repository.

## TODO

- [ ] During implementation, run a real `glaze help export --format sqlite` smoke test and paste the observed schema into follow-up notes if it differs from `sections`.
- [x] Decide final command location and name before coding.
- [x] Decide whether `outputDB` is writable scratch, read-only comparison data, or a copy of `inputDB`.
- [x] Confirm implementation scope: app binary named chat with embedded help entries only
- [ ] Scaffold Go module and chat command skeleton
- [ ] Embed programmatic Glazed help entries and materialize them into the input SQLite DB
- [ ] Implement SQLite DB facades for JavaScript globals inputDB and outputDB
- [ ] Implement scopedjs eval_js runtime/tool registrar
- [ ] Wire Pinocchio profile resolution, Geppetto runner, and stdin/stdout REPL
- [ ] Add unit/smoke tests for help DB materialization and eval_js DB access
- [ ] Run formatting/tests, update diary/bookkeeping, and commit implementation
