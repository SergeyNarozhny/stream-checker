# Postgresql notify-listen golang stream checker

Used with postgresql trigger and procedure, e.g.:

```
CREATE OR REPLACE FUNCTION t2.notifystream() RETURNS trigger AS $$
DECLARE
BEGIN
 IF TG_OP != 'DELETE' THEN
  PERFORM pg_notify('streamwatcher', row_to_json(NEW)::text);
  RETURN new;
 ELSE
  PERFORM pg_notify('streamwatcher', 'delete_' || OLD.id);
  RETURN NULL;
 END IF;
END;
$$ LANGUAGE plpgsql;
```
and

```
CREATE TRIGGER triggerNotifyStream
  AFTER INSERT OR UPDATE OR DELETE
  ON t2.stream
  FOR EACH ROW
  EXECUTE PROCEDURE t2.notifystream();
```
