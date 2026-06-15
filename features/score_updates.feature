Feature: Real-Time Score Updates
  In order to know where I stand instantly
  As a participant in a quiz session
  I want my score updated and broadcast the moment I answer

  # NOTE (e2e adaptation): scores change only via correct answers (+10 each), so
  # preset scores are reached by answering N questions correctly. Answers that must
  # be observed by OTHER clients are sent over WebSocket (REST submission returns
  # the result to the caller but does not broadcast). Latency ("within 500ms") is
  # advisory — see docs/05-e2e-test-plan.md. Sessions here use 5 questions.

  Background:
    Given a quiz session "QUIZ-ABC" is active

  Scenario: Score is updated immediately upon a correct answer
    Given "Alice" is a participant with score 10
    When "Alice" submits a correct answer
    Then "Alice"'s score becomes 20
    And the score update is broadcast to all participants

  Scenario: Score broadcast reaches all connected participants
    Given "Alice" is a participant with score 10
    And "Bob" is connected
    And "Charlie" is connected
    When "Alice" submits a correct answer and her score becomes 20
    Then "Bob" receives the score update for "Alice"
    And "Charlie" receives the score update for "Alice"

  Scenario: Scoring is accurate — identical inputs produce identical scores
    Given "Alice" and "Bob" both have score 0
    When "Alice" submits a correct answer for question "Q1"
    And "Bob" submits a correct answer for question "Q1"
    Then both "Alice" and "Bob" have score 10

  Scenario: Scores do not change retroactively
    Given "Alice" has score 30 after answering 3 questions correctly
    When a new question "Q4" is broadcast
    Then "Alice"'s score remains 30 until she answers "Q4"

  Scenario: Score is not increased for an incorrect answer
    Given "Alice" is a participant with score 10
    When "Alice" submits an incorrect answer
    Then "Alice"'s score remains 10
