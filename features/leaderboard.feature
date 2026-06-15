Feature: Real-Time Leaderboard
  In order to feel the thrill of competition
  As a participant in a quiz session
  I want a live ranked leaderboard that updates the instant any score changes

  # NOTE (e2e adaptation): scores are reached by answering N questions correctly
  # (+10 each), so the docs' illustrative absolute scores (30/40/50/60) become
  # "answered correctly: N". Ranking is asserted by participant order; tie-breaking
  # is exercised by having players reach the same score in a known sequence.
  # Latency ("within 500ms") is advisory. See docs/05-e2e-test-plan.md.

  Background:
    Given a quiz session "QUIZ-ABC" is active

  # ----- Happy Path -----

  Scenario: Leaderboard displays correct rankings
    Given the participants have answered correctly:
      """
      Bob: 5
      Charlie: 4
      Alice: 3
      """
    When the leaderboard is calculated
    Then the leaderboard ranks the participants:
      """
      1. Bob
      2. Charlie
      3. Alice
      """

  Scenario: Leaderboard updates when a score changes
    Given the participants have answered correctly:
      """
      Bob: 2
      Charlie: 1
      Alice: 0
      """
    When "Alice" answers 3 questions correctly
    Then the leaderboard ranks the participants:
      """
      1. Alice
      2. Bob
      3. Charlie
      """

  Scenario: Leaderboard is broadcast to all connected participants
    Given 3 participants are connected
    When any connected participant's score changes
    Then all 3 participants receive the updated leaderboard

  # ----- Tie-Breaking -----

  Scenario: Tie-breaking by earlier submission time
    Given "Alice" reaches score 10 before "Bob"
    When the leaderboard is calculated
    Then "Alice" is ranked above "Bob"

  Scenario: Multiple participants tied at the same score
    Given "Alice", "Bob", and "Charlie" each reach score 10 in that order
    When the leaderboard is calculated
    Then the leaderboard ranks the participants:
      """
      1. Alice
      2. Bob
      3. Charlie
      """
