DROP TABLE IF EXISTS posts;

CREATE TABLE posts (
	id      SERIAL PRIMARY KEY, 
	header  VARCHAR NOT NULL,  -- The title of the Post
	content TEXT NOT NULL,     -- The content of the blog post
	slug    VARCHAR UNIQUE NOT NULL   -- The url we access this post on
);