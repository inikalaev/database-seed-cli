-- Golden-file schema for end-to-end regression testing.
-- Every archetype below exercises a distinct code path:
--
--   customers    — SERIAL PK, text/email/name factories, timestamptz, UNIQUE.
--   categories   — self-FK (tree) → cycle detection + SET CONSTRAINTS ALL DEFERRED.
--   products     — UUID PK, JSONB, BOOLEAN, TEXT[], DATE, BYTEA, NUMERIC(p,s),
--                  FK to categories.
--   orders       — composite-like FK graph (customers + products), enum column,
--                  row_count_per exercised via config.

CREATE TYPE public.order_status AS ENUM ('pending', 'paid', 'shipped');

CREATE TABLE public.customers (
    id SERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE public.categories (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    parent_id INTEGER REFERENCES public.categories(id) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE public.products (
    id UUID PRIMARY KEY,
    category_id INTEGER NOT NULL REFERENCES public.categories(id),
    name TEXT NOT NULL,
    price NUMERIC(10, 2) NOT NULL,
    active BOOLEAN NOT NULL,
    released_at DATE NOT NULL,
    tags TEXT[] NOT NULL,
    metadata JSONB,
    signature BYTEA
);

CREATE TABLE public.orders (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER NOT NULL REFERENCES public.customers(id),
    product_id UUID NOT NULL REFERENCES public.products(id),
    status public.order_status NOT NULL,
    total_amount NUMERIC(10, 2) NOT NULL,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL
);
