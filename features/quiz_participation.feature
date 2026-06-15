Feature: Real-Time Quiz Participation
  In order to compete fairly in real time
  As a participant in a quiz session
  I want to receive questions and submit answers as the quiz runs

  Background:
    Given a quiz session "QUIZ-ABC" is active
    And "Alice" is a participant in session "QUIZ-ABC"

  # ----- Happy Path -----

  Scenario: Submitting a correct answer
    Given the current question is:
      """
      {
        "id": "Q1",
        "text": "past tense of 'go'",
        "options": ["goed", "went", "gone", "going"],
        "correctAnswer": "went"
      }
      """
    When "Alice" submits answer "went" for question "Q1"
    Then the answer is marked as correct
    And "Alice"'s score is increased by the base points

  Scenario: Submitting an incorrect answer
    Given the current question has correct answer "went" for question "Q1"
    When "Alice" submits answer "goed" for question "Q1"
    Then the answer is marked as incorrect
    And "Alice"'s score is not increased

  Scenario: Broadcasting a question to all participants
    Given the following participants are connected:
      """
      Alice
      Bob
      Charlie
      """
    When the next question is broadcast
    Then all 3 participants receive the question simultaneously
    And each participant can submit an answer

  # ----- Edge Cases -----

  Scenario: Preventing duplicate answer submissions
    Given "Alice" has already submitted answer "went" for question "Q1"
    When "Alice" submits another answer "gone" for question "Q1"
    Then the system rejects the duplicate submission
    And "Alice"'s score remains unchanged
    And the system returns an error "Answer already submitted for this question"

  Scenario: Submitting an answer for a non-existent question
    When "Alice" submits answer "went" for question "Q-NONEXISTENT"
    Then the system returns an error "Question not found"
    And "Alice"'s score remains unchanged

  Scenario: Submitting an answer without having joined the session
    Given "Mallory" is not a participant in session "QUIZ-ABC"
    When "Mallory" submits answer "went" for question "Q1"
    Then the system returns an error "Participant has not joined this quiz session"
    And no score is recorded for "Mallory"
