---
Title: Database Globals API
Slug: database-globals-api
Short: API reference for inputDB and outputDB in the chat JavaScript runtime.
Topics:
  - chat
  - sqlite
  - javascript
Commands:
  - chat
Flags: []
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
Order: 20
---

# Database Globals API

The `chat` agent exposes two SQLite facades to JavaScript.

## inputDB

`inputDB` is read-only. It contains help sections embedded in the `chat` binary and registered at startup.

Methods:

- `inputDB.query(sql, ...args)`: execute a `SELECT` or `WITH` query and return rows as objects.
- `inputDB.schema()`: return known tables/views and useful columns.

Example:

```javascript
const rows = inputDB.query(`
  SELECT slug, title, short
  FROM docs
  WHERE content LIKE ?
  ORDER BY title
  LIMIT 10
`, "%eval_js%");
return rows;
```

## outputDB

`outputDB` is writable scratch space for the current process.

Methods:

- `outputDB.query(sql, ...args)`: execute a read query.
- `outputDB.exec(sql, ...args)`: execute a write statement and return `{ rowsAffected, lastInsertId }`.
- `outputDB.schema()`: return known scratch tables.

The default scratch schema contains `notes`:

```sql
CREATE TABLE notes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  key TEXT,
  value TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```
