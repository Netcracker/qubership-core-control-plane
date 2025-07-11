CREATE TABLE IF NOT EXISTS lua_filters
(
    id              serial primary key,
    name            varchar(100) not null unique,
    url             text not null,
    header_name     varchar(255) not null,
    lua_script      text not null,
    is_active       boolean not null default true, 
    sha256          varchar(64),
    timeout         int,
    params          jsonb
);

CREATE TABLE IF NOT EXISTS listeners_lua_filters
(
    listener_id    int not null,
    lua_filter_id  int not null,
    PRIMARY KEY (listener_id, lua_filter_id)
);
