-- Imported snapshots can contain explicit IDs without advancing their serial
-- sequences. Advance only lagging sequences; never reuse an already issued ID.
DO $$
DECLARE
    item record;
    sequence_name text;
    maximum_id bigint;
    sequence_value bigint;
    sequence_called boolean;
BEGIN
    FOR item IN
        SELECT table_schema, table_name, column_name
        FROM information_schema.columns
        WHERE table_schema = 'public' AND column_default LIKE 'nextval(%'
        ORDER BY table_name, column_name
    LOOP
        sequence_name := pg_get_serial_sequence(format('%I.%I', item.table_schema, item.table_name), item.column_name);
        IF sequence_name IS NULL THEN
            CONTINUE;
        END IF;
        -- Prevent an INSERT from racing the max-ID read and sequence update.
        EXECUTE format('LOCK TABLE %I.%I IN SHARE ROW EXCLUSIVE MODE', item.table_schema, item.table_name);
        EXECUTE format('SELECT max(%I) FROM %I.%I', item.column_name, item.table_schema, item.table_name) INTO maximum_id;
        EXECUTE format('SELECT last_value, is_called FROM %s', sequence_name) INTO sequence_value, sequence_called;
        IF maximum_id > sequence_value OR (maximum_id = sequence_value AND NOT sequence_called) THEN
            PERFORM setval(sequence_name::regclass, maximum_id, true);
        END IF;
    END LOOP;
END
$$;
