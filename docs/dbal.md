# DBAL

`internal/dbal` owns logical Select/Insert/Update/Delete operations, predicates, joins, grouping, aggregates, order, and bounded pagination. Values are always parameters and identifiers are validated. Portable error codes include conflict, unique/foreign-key violation, not found, busy, invalid query, and internal.

`internal/dbal/sqlite` alone imports the driver and compiles SQL. It enables foreign keys, WAL, a busy timeout, bounded connections, affected-row checks, transaction callbacks, and schema inspection. Migration SQL is isolated under `internal/migration`.
