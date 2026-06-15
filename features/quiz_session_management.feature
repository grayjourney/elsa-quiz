Feature: Quiz Session Management
  In order to play together in the same game
  As a learner using the ELSA app
  I want to create and join quiz sessions using a unique quiz ID

  # NOTE (e2e adaptation): quiz IDs are server-generated, so the literal IDs below
  # (QUIZ-ABC, QUIZ-OLD, …) are labels — the harness binds them to the session
  # actually created in the scenario. QUIZ-999 (never created) stays a genuinely
  # non-existent id. See docs/05-e2e-test-plan.md §"Deviations from docs/02".

  Background:
    Given the quiz server is running

  # ----- Happy Path -----

  Scenario: Creating a new quiz session
    When a host requests to create a new quiz session
    Then the system generates a unique quiz ID
    And the session status is "waiting"
    And the session is available for users to join

  Scenario: Joining a quiz session with a valid ID
    Given a quiz session exists with ID "QUIZ-ABC"
    And the session status is "waiting"
    When a user "Alice" joins the session with ID "QUIZ-ABC"
    Then "Alice" is added to the session's participant list
    And "Alice" receives a confirmation of successful join
    And "Alice"'s initial score is 0

  Scenario: Multiple users joining the same session simultaneously
    Given a quiz session exists with ID "QUIZ-ABC"
    When the following users join session "QUIZ-ABC" simultaneously:
      """
      Alice
      Bob
      Charlie
      Diana
      Eve
      """
    Then all 5 users are added to the session's participant list
    And each user receives a confirmation of successful join
    And the session has exactly 5 participants

  Scenario: Notifying existing participants when a new user joins
    Given a quiz session exists with ID "QUIZ-ABC"
    And "Alice" is already a participant in session "QUIZ-ABC"
    When "Bob" joins the session with ID "QUIZ-ABC"
    Then "Alice" receives a notification that "Bob" has joined
    And the participant count is updated to 2

  Scenario: Joining a quiz session already in progress (late join)
    Given a quiz session exists with ID "QUIZ-ABC"
    And the session status is "active"
    And 2 questions have already been broadcast
    When a user "Frank" joins the session with ID "QUIZ-ABC"
    Then "Frank" is added to the session's participant list
    And "Frank"'s initial score is 0
    And "Frank" receives the current question
    And "Frank" can submit answers from the current question onward

  # ----- Error Cases -----

  Scenario: Joining a non-existent quiz session
    Given no quiz session exists with ID "QUIZ-999"
    When a user "Alice" tries to join session "QUIZ-999"
    Then the join is rejected with error code "session_not_found"
    And "Alice" is not added to any session

  Scenario: Joining an already completed quiz session
    Given a quiz session exists with ID "QUIZ-OLD"
    And the session status is "completed"
    When a user "Alice" tries to join session "QUIZ-OLD"
    Then the join is rejected with error code "session_ended"
    And "Alice" is not added to any session

  Scenario: Joining with an empty quiz ID
    When a user "Alice" tries to join session ""
    Then the join is rejected with error code "quiz_id_required"
    And "Alice" is not added to any session
