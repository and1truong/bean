# ADR 0010: Source-owned shadcn/ui system

Status: accepted.

Bean uses checked-in shadcn/ui components with Tailwind CSS and CSS-variable Bean tokens for every frontend surface owned by the runtime: Shell/Auth, public metadata rendering, Application Admin, System Admin, and Studio.

The components live under `web/src/components/ui` and compile into the existing embedded Vite bundle. shadcn is a source distribution model, not a runtime UI service, so the single-executable deployment contract remains unchanged.

Dynamic forms use the shadcn native select primitive, including its adapted multiple-select mode, to preserve browser validation and metadata-driven option handling. Admin tables compose the shadcn Table primitive around Bean's existing server-side search, filtering, sorting, and cursor pagination rather than adding a second client table state model. Validation remains metadata/server-driven; Bean does not add a parallel form schema.

Destructive or metadata-confirmed operations use AlertDialog. Raw interactive controls and tables are disallowed outside the checked-in primitive directory so new Bean-owned UI follows the same accessible design system.
