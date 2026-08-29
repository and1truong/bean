# Definitions

Definitions use `apiVersion: bean/v1alpha1`, a supported `kind`, lowercase namespaced metadata, and a typed specification. Supported kinds are Entity, View, Action, Webform, Policy, Block, Panel, Page, Role, Menu, and Job. Bundle files contain `name`, `definitions`, and optional seed rows.

Compilation validates envelopes, names, fields, references, relation kinds, limits, Action steps, Panel regions, and route uniqueness. Diagnostics identify kind, name, specification path, and a corrective message. Generated CRUD is emitted as Views and Actions inside AppIR.
