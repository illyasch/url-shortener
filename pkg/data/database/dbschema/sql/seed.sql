INSERT INTO urls (url, date_created) VALUES
	('https://www.cnn.com', '2019-03-24 00:00:00')
	ON CONFLICT DO NOTHING;