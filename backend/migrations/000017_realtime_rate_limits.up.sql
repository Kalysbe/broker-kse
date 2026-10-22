CREATE TABLE realtime_rate_limits (
 scope TEXT NOT NULL CHECK (scope IN ('global','broker')),
 subject TEXT NOT NULL,
 operation TEXT NOT NULL CHECK (operation IN ('*','System','Connect','Subscribe','Unsubscribe','Ping','UnknownCommand')),
 rate INTEGER NOT NULL CHECK(rate BETWEEN 1 AND 1000000),
 burst INTEGER NOT NULL CHECK(burst BETWEEN 1 AND 1000000),
 concurrent INTEGER NOT NULL CHECK(concurrent BETWEEN 1 AND 1000000),
 enabled BOOLEAN NOT NULL DEFAULT true,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(scope,subject,operation)
);
INSERT INTO realtime_rate_limits(scope,subject,operation,rate,burst,concurrent) VALUES
 ('global','*','System',1000,200,256),
 ('broker','*','System',100,50,64),
 ('broker','*','Connect',5,10,8),
 ('broker','*','Subscribe',20,10,32),
 ('broker','*','Unsubscribe',20,10,32),
 ('broker','*','Ping',100,50,32);
