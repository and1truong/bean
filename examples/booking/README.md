# Booking

A resource-booking application that demonstrates deterministic Rules, transaction Actions, overlap protection, scheduled Jobs, semantic tests, calendar rendering, and generated demo data.

## Highlights

- Resources and bookings
- Derived request timestamps and interval validation
- Atomic booking with overlap detection
- Cancellation transition and reminder Job
- Calendar View and home Page

## Definition layout

- `app.yaml` — application entry point
- `resources.yaml` — bookable resource model
- `bookings.yaml` — booking model, Rules, Actions, Job, semantic tests, and demo data
- `calendar.yaml` — calendar View and home-page composition

## Run it

From the repository root:

```bash
./bin/bean app validate --file ./examples/booking/app.yaml
./bin/bean app publish --file ./examples/booking/app.yaml --db ./tmp/booking.db --json
./bin/bean demo seed --file ./examples/booking/app.yaml --db ./tmp/booking.db --seed 42 --json
./bin/bean serve --db ./tmp/booking.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/>. Run the Action contract tests with:

```bash
./bin/bean app test --file ./examples/booking/app.yaml --json
```
