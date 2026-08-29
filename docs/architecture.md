# Architecture

Bean is a modular Go monolith with an embedded React application and a single SQLite database.

```text
YAML/Studio drafts -> validate -> dependency graph -> additive migration plan
                  -> immutable AppIR -> atomic activation -> HTTP runtime

HTTP -> Context/Policy -> View (reads) / Action (writes) -> typed DBAL -> SQLite
UI   <- typed render tree / manifest <- Page <- Panel <- Block
```

Definitions and releases are persisted, but normal requests read the active immutable AppIR from memory. Publication applies safe schema changes in a transaction, persists the compiled release, and swaps the active pointer only after commit. Generated CRUD uses the same View and Action engines as domain metadata.

The DBAL owns logical queries and portable errors. SQLite SQL is confined to its adapter and the migration executor. Authentication uses database sessions; policies contribute both route decisions and row predicates. Actions own validation, authorization, transaction, audit, and outbox behavior.
