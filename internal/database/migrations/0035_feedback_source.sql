ALTER TABLE pangu.feedback ADD COLUMN source text NOT NULL DEFAULT 'unknown' CHECK(source IN ('unknown','feishu_session'));
