Feature: Operations and Observability
  In order to run the service reliably in production
  As the platform operator
  I want health and metrics endpoints to monitor the server

  # NOTE (e2e adaptation): the server is shared across the whole black-box run and
  # its session/metric counters are process-global, so these scenarios assert
  # "at least" thresholds and seed their own activity rather than exact totals.
  # See docs/05-e2e-test-plan.md §"Deviations from docs/02".

  Scenario: Health check reports liveness and active sessions
    Given the quiz server is running
    And 2 quiz sessions are active
    When a client requests "GET /api/health"
    Then the response status is 200
    And the body reports status "ok"
    And the body reports an active session count of at least 2

  Scenario: Metrics endpoint exposes quiz metrics
    Given the quiz server is running
    And a quiz session has processed at least one answer
    When a client requests "GET /metrics"
    Then the response status is 200
    And the body is in Prometheus exposition format
    And it includes "quiz_active_sessions", "quiz_connected_users", and "quiz_answers_total"
