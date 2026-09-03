# CMS / News

A compact editorial CMS example. It demonstrates article publishing, category and author records, an administration resource, scheduled publication, and multiple serialized forms of one public View.

## Highlights

- Draft and published articles
- Editorial `publish_article` transition Action
- JSON, CSV, and RSS displays
- Public news Page and administration resource
- Scheduled publishing Job

## Run it

From the repository root:

```bash
./bin/bean app validate --file ./examples/cms/app.yaml
./bin/bean app publish --file ./examples/cms/app.yaml --db ./tmp/cms.db --json
./bin/bean serve --db ./tmp/cms.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/> for published news or <http://127.0.0.1:8080/admin> to manage content.
