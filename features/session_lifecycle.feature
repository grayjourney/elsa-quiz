Feature: Session Lifecycle
  In order to run a quiz from start to finish without stalling on absent players
  As a quiz host
  I want to choose how the quiz advances and ends — host-controlled or time-limited

  # An end policy is chosen at session creation and governs BOTH question
  # advancement and quiz termination:
  #   * manual — the host advances each question and ends the quiz on demand
  #   * timed  — each question has a time limit; advancement and end are automatic
  #
  # Invariant (both policies): progress depends only on ACTIVE (connected)
  # participants and/or the clock — never on a specific participant answering.
  #
  # NOTE (e2e adaptation): timed limits use 1s (REST contract is whole seconds; the
  # docs' illustrative 30s would make the suite slow); quizzes use 3 questions; and
  # rejections assert the machine error code rather than the display message.
  # See docs/05-e2e-test-plan.md.

  # ----- Configuring the End Policy at Creation -----

  Scenario: Creating a session with the manual end policy
    Given the quiz server is running
    When a host creates a new quiz session with end policy "manual"
    Then the session is created with end policy "manual"
    And the session status is "waiting"

  Scenario: Creating a session with the timed end policy
    Given the quiz server is running
    When a host creates a new quiz session with end policy "timed" and a per-question time limit of 1 second
    Then the session is created with end policy "timed"
    And each question carries a time limit of 1 second

  Scenario: Rejecting a timed session with a non-positive time limit
    Given the quiz server is running
    When a host creates a new quiz session with end policy "timed" and a per-question time limit of 0 seconds
    Then the request is rejected with error code "invalid_time_limit"
    And no session is created

  # ----- Quiz Start (common to both policies) -----

  Scenario: Starting a quiz session
    Given a quiz session "QUIZ-ABC" exists with status "waiting"
    And 5 participants have joined
    When the host starts the quiz
    Then the session status changes to "active"
    And the first question is broadcast to all participants

  # ----- Manual Policy: Host-Controlled Advancement & End -----

  Scenario: Host advances to the next question
    Given a quiz session "QUIZ-ABC" is active with end policy "manual"
    And the current question is "Q1"
    When the host advances to the next question
    Then question "Q2" is broadcast to all participants
    And no further answers are accepted for "Q1"

  Scenario: Host ends the quiz while a participant is AFK
    Given a quiz session "QUIZ-ABC" is active with end policy "manual"
    And the participants are:
      """
      Alice   (answered)
      Bob     (answered)
      Charlie (AFK)
      """
    When the host ends the quiz
    Then the session status changes to "completed"
    And the final leaderboard is displayed to all participants
    And "Charlie" is scored 0 for the unanswered question
    And no further answer submissions are accepted

  Scenario: Quiz completes after the host advances past the final question
    Given a quiz session "QUIZ-ABC" is active with end policy "manual"
    And the current question is the final question "Q3"
    When the host advances past "Q3"
    Then the session status changes to "completed"
    And the final leaderboard is displayed to all participants

  # ----- Timed Policy: Automatic Advancement & End -----

  Scenario: Question auto-advances when its time limit expires
    Given a quiz session "QUIZ-ABC" is active with end policy "timed" and a 1 second time limit
    And the current question "Q1" has been open
    And "Charlie" has not answered "Q1"
    When the time limit for "Q1" expires
    Then question "Q2" is broadcast to all participants
    And "Charlie" is scored 0 for "Q1"
    And no further answers are accepted for "Q1"

  Scenario: Question advances early when all active participants have answered
    Given a quiz session "QUIZ-ABC" is active with end policy "timed" and a 30 second time limit
    And 3 participants are connected
    And the current question is "Q1"
    When all 3 connected participants submit an answer for "Q1" before the time limit
    Then question "Q2" is broadcast without waiting for the timer

  Scenario: An AFK participant never blocks a timed quiz from ending
    Given a quiz session "QUIZ-ABC" is active with end policy "timed" and a 1 second time limit
    And the quiz has 3 questions total
    And "Charlie" goes AFK after question 1
    When the time limit expires for each remaining question
    Then the session status changes to "completed"
    And the final leaderboard is displayed to all participants

  Scenario: Rejecting a late answer after a question's time limit expired
    Given a quiz session "QUIZ-ABC" is active with end policy "timed" and a 1 second time limit
    And "Alice" is a participant
    And the time limit for question "Q1" has expired
    When "Alice" submits an answer for "Q1"
    Then the request is rejected with error code "time_up"
    And "Alice"'s score remains unchanged

  # ----- End Guards (common to both policies) -----

  Scenario: Rejecting answer submissions after the quiz ends
    Given a quiz session "QUIZ-ABC" has status "completed"
    And "Alice" is a participant
    When "Alice" tries to submit an answer
    Then the request is rejected with error code "quiz_ended"
    And "Alice"'s score remains unchanged

  Scenario: Ending an already-completed quiz is rejected
    Given a quiz session "QUIZ-ABC" has status "completed"
    When the host ends the quiz again
    Then the request is rejected with error code "quiz_ended"
    And the session remains "completed"
