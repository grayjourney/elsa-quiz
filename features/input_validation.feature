Feature: Input Validation
  In order to keep scoring trustworthy
  As the platform operator
  I want malformed or empty answers rejected with clear errors

  Background:
    Given a quiz session "QUIZ-ABC" is active
    And "Alice" is a participant

  Scenario: Rejecting empty answer submission
    When "Alice" submits an empty answer "" for question "Q1"
    Then the system returns an error "Answer cannot be empty"
    And the response status is 400

  Scenario: Rejecting answer with invalid format
    When "Alice" submits an answer that is not one of the valid options
    Then the system returns an error "Invalid answer option"
    And the response status is 400
