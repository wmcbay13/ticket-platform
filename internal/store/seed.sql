INSERT INTO events (id, title, venue, starts_at, description, price_cents, capacity, available) VALUES
('neon-nights', 'Neon Nights', 'The Foundry · Brooklyn', '2027-05-14 23:00:00+00', 'An evening of electronic music, live visuals, and unexpected collaborations.', 4500, 5000, 5000),
('cloud-native-live', 'Cloud Native Live', 'Pier 36 · New York', '2027-06-05 14:00:00+00', 'Meet the people building the next wave of infrastructure. Talks, demos, and good coffee.', 7900, 10000, 10000),
('rooftop-sessions', 'Rooftop Sessions', 'Skyline Terrace · Manhattan', '2027-06-19 22:00:00+00', 'Soul, jazz, and sunset views from the city’s favorite rooftop.', 3500, 3000, 3000)
ON CONFLICT (id) DO NOTHING;
