# Delta for Payments

## ADDED Requirements
### Requirement: Payment retries on timeout
The system SHALL retry a timed-out payment up to three times.

#### Scenario: Timeout
- **WHEN** the gateway times out
- **THEN** the client retries

## REMOVED Requirements
### Requirement: Retry policy
The system SHALL NOT retry.
