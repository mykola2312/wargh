CREATE TABLE user (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    login       TEXT    NOT NULL UNIQUE,
    password    TEXT    NOT NULL
);
