-- Profile Hub schema — mirrors Supabase portfolio tables.
-- Auto-run on first container start via docker-entrypoint-initdb.d.

SET statement_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;

CREATE TABLE brands (
    id uuid PRIMARY KEY,
    name text,
    logo text,
    color text,
    display_order integer,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE case_studies (
    id uuid PRIMARY KEY,
    case_id text,
    title text,
    role text,
    challenge text,
    solution text,
    impact text,
    metrics jsonb,
    tech_stack jsonb,
    display_order integer,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE chat_questions (
    id uuid PRIMARY KEY,
    question_id text,
    text text,
    response text,
    display_order integer,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE education (
    id uuid PRIMARY KEY,
    institution text,
    degree text,
    period text,
    start_date text,
    end_date text,
    location text,
    focus_area text,
    description text,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE experience (
    id uuid PRIMARY KEY,
    company text,
    role text,
    period text,
    start_date text,
    end_date text,
    location text,
    experience_type text,
    description text,
    achievements jsonb,
    details jsonb,
    tags jsonb,
    remote_work boolean,
    team_size integer,
    tech_stack jsonb,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    clients jsonb,
    managerial_achievements jsonb,
    ai_enablement jsonb,
    key_metrics jsonb
);

CREATE TABLE faqs (
    id uuid PRIMARY KEY,
    question text,
    answer text,
    display_order integer,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE projects (
    id uuid PRIMARY KEY,
    title text,
    description text,
    short_description text,
    image text,
    thumbnail text,
    tech_stack jsonb,
    link text,
    category text,
    company text,
    clients text,
    start_date text,
    end_date text,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE services (
    id uuid PRIMARY KEY,
    title text,
    description text,
    icon_name text,
    skills jsonb,
    price_range text,
    timeline text,
    features jsonb,
    service_type text,
    display_order integer,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE site_config (
    id uuid PRIMARY KEY,
    name text,
    title text,
    email text,
    short_bio text,
    long_bio jsonb,
    location text,
    years_experience_start_year integer,
    whatsapp text,
    contact_info jsonb,
    social_media jsonb,
    domain_expertise jsonb,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE skill_categories (
    id uuid PRIMARY KEY,
    category_name text,
    skills jsonb,
    display_order integer,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE skill_radar_data (
    id uuid PRIMARY KEY,
    subject text,
    value integer,
    full_mark integer,
    display_order integer,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE stats_dashboard (
    id uuid PRIMARY KEY,
    label text,
    value text,
    icon text,
    display_order integer,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE testimonials (
    id uuid PRIMARY KEY,
    name text,
    "position" text,
    company text,
    text text,
    image text,
    location text,
    display_order integer,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);
