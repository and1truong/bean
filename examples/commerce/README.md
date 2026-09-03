# Commerce

A commerce operations example covering products, inventory, orders, and fulfillment. It demonstrates calculated fields, transactional Actions, lifecycle transitions, after-commit HTTP Extensions, operational dashboards, semantic tests, and generated demo data.

## Highlights

- Inventory-aware order placement
- Derived order-item totals
- Order fulfillment lifecycle
- Typed, retryable notification Extension
- Inventory and fulfillment metrics and charts

## Run it

From the repository root:

```bash
./bin/bean app validate --file ./examples/commerce/app.yaml
./bin/bean app publish --file ./examples/commerce/app.yaml --db ./tmp/commerce.db --json
./bin/bean demo seed --file ./examples/commerce/app.yaml --db ./tmp/commerce.db --seed 42 --json
./bin/bean serve --db ./tmp/commerce.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/>. Run the semantic contracts with:

```bash
./bin/bean app test --file ./examples/commerce/app.yaml --json
```

The notification endpoint uses the reserved `.example` domain and is illustrative; configure a real endpoint before invoking that Extension outside tests.
