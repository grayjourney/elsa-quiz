Feature: Connection Management and Reliability
  In order to not lose my progress when my network hiccups
  As a participant in a quiz session
  I want my state preserved across disconnects and concurrent activity

  # NOTE (e2e adaptation): scores are reached by answering correctly (+10). The two
  # high-concurrency scenarios are tagged @perf (advisory tier) — data-race safety
  # is already a BLOCKING gate via `go test -race` at the unit layer; here they run
  # as a non-blocking smoke check. See docs/05-e2e-test-plan.md.

  Background:
    Given a quiz session "QUIZ-ABC" is active

  # ----- Disconnection Handling (Tier 1) -----

  Scenario: Preserving state on client disconnection
    Given "Alice" is a participant with score 30
    When "Alice"'s network connection drops
    Then "Alice"'s score of 30 is preserved in the session
    And the leaderboard continues to show "Alice" with score 30

  Scenario: Reconnecting after disconnection
    Given "Alice" was a participant with score 30
    And "Alice"'s connection was dropped
    When "Alice" reconnects to session "QUIZ-ABC"
    Then "Alice" resumes with her previous score of 30
    And "Alice" can continue submitting answers

  # ----- Concurrent Access (advisory) -----

  @perf
  Scenario: Handling simultaneous answer submissions without race conditions
    Given 100 participants are connected
    When all 100 participants submit answers for question "Q1" simultaneously
    Then all scores are calculated correctly
    And no scores are lost or duplicated
    And the leaderboard reflects accurate rankings

  @perf
  Scenario: No data corruption under concurrent writes
    Given "Alice" submits an answer at the exact same time as "Bob"
    When both answers are processed
    Then "Alice"'s score reflects only her answer
    And "Bob"'s score reflects only his answer
