CREATE TABLE IF NOT EXISTS lua_filters
(
    id              serial primary key,
    name            varchar(100) not null unique,
    lua_script      text not null
);

ALTER TABLE routes ADD COLUMN IF NOT EXISTS lua_filter_name text;