INSERT INTO urls (id, url, date_created) VALUES
	(1, 'https://www.cnn.com', '2019-03-24 00:00:00')
	ON CONFLICT DO NOTHING;