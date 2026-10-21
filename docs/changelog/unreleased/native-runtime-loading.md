### Fixed

- Native runtime reads reuse a validated binding across joined cases in one read, reducing repeated JSON decoding while still checking the active receipt and bound inputs on each new read.
