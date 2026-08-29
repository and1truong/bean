# Creating an application

Start with an Entity and fields, then expose reads through Views and writes through Actions. A Webform submits to an Action. Public screens are Pages whose Panel regions contain Blocks. Add Policies to all non-public reads and writes.

Import with `bean app import`, run `bean validate`, review diagnostics/migration output in Studio, and run `bean publish`. Machine names are permanent after publication; additive fields, indexes, relations, labels, and presentation changes are supported. Destructive or incompatible changes are rejected.
