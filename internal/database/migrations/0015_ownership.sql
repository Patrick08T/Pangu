ALTER TABLE pangu.submissions ADD COLUMN created_by uuid REFERENCES pangu.users(id);
ALTER TABLE pangu.schedules ADD COLUMN created_by uuid REFERENCES pangu.users(id);
CREATE INDEX schedules_creator ON pangu.schedules(created_by) WHERE NOT deleted;
