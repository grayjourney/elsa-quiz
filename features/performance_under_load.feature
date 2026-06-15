@perf
Feature: Performance Under Load
  In order to keep the experience real-time at scale
  As the platform operator
  I want latency and throughput targets met under heavy load

  # NOTE (e2e adaptation): this is the ADVISORY tier — failures WARN, never block
  # (see docs/05-e2e-test-plan.md). Counts are scaled to run cleanly within the
  # harness's WebSocket client buffers; latency is measured and reported. Rigorous
  # load/soak testing is a separate concern from this functional smoke check.

  Scenario: Maintaining broadcast latency under load
    Given a quiz session "QUIZ-ABC" is active
    And 25 participants are connected
    When each connected participant submits a correct answer
    Then every score update is broadcast within 500ms at p95

  Scenario: Server handles high message throughput
    Given 50 concurrent quiz sessions each with 20 participants
    When all participants submit answers simultaneously
    Then the server processes every submission without dropping any
