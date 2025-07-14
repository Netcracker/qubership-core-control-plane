CREATE TABLE IF NOT EXISTS lua_filters
(
    id              serial primary key,
    name            varchar(100) not null unique,
    lua_script      text not null
);

CREATE TABLE IF NOT EXISTS listeners_lua_filters
(
    listener_id    int not null,
    lua_filter_id  int not null,
    PRIMARY KEY (listener_id, lua_filter_id)
);

ALTER TABLE routes ADD COLUMN IF NOT EXISTS lua_filter_id text;