# Test Cases — Real-Time Vocabulary Quiz

**Version:** 1.2
**Date:** 2026-06-13
**Derived From:** [Product Requirements Document](./01-product-requirements.md)
**Testing Strategy:** BDD-style scenarios (Gherkin) covering all acceptance criteria

---

## How to Read This Document

Each feature below is written as a standalone **Gherkin feature file**, the same
format consumed by `godog` (Cucumber for Go). Every feature opens with a
narrative (`In order to… / As a… / I want…`) that states the business value,
followed by an optional `Background` of shared preconditions, then the scenarios.
The `.feature` blocks here map 1:1 to the files under `features/` in the
implementation plan.

> **Keyword alignment** follows the Gherkin convention: `Given / When / Then`,
> with `And` / `But` continuing the previous step. Doc-strings (`"""…"""`) carry
> tabular or multi-line example data.

---

## Feature: Quiz Session Management

```gherkin
Feature: Quiz Session Management
  In order to play together in the same game
  As a learner using the ELSA app
  I want to create and join quiz sessions using a unique quiz ID

  Background:
    Given the quiz server is running

  # ----- Happy Path -----

  Scenario: Creating a new quiz session
    When a host requests to create a new quiz session
    Then the system generates a unique quiz ID
    And  the session status is "waiting"
    And  the session is available for users to join

  Scenario: Joining a quiz session with a valid ID
    Given a quiz session exists with ID "QUIZ-ABC"
    And   the session status is "waiting"
    When  a user "Alice" joins the session with ID "QUIZ-ABC"
    Then  "Alice" is added to the session's participant list
    And   "Alice" receives a confirmation of successful join
    And   "Alice"'s initial score is 0

  Scenario: Multiple users joining the same session simultaneously
    Given a quiz session exists with ID "QUIZ-ABC"
    When  the following users join session "QUIZ-ABC" simultaneously:
      """
      Alice
      Bob
      Charlie
      Diana
      Eve
      """
    Then  all 5 users are added to the session's participant list
    And   each user receives a confirmation of successful join
    And   the session has exactly 5 participants

  Scenario: Notifying existing participants when a new user joins
    Given a quiz session exists with ID "QUIZ-ABC"
    And   "Alice" is already a participant in session "QUIZ-ABC"
    When  "Bob" joins the session with ID "QUIZ-ABC"
    Then  "Alice" receives a notification that "Bob" has joined
    And   the participant count is updated to 2

  Scenario: Joining a quiz session already in progress (late join)
    Given a quiz session exists with ID "QUIZ-ABC"
    And   the session status is "active"
    And   2 questions have already been broadcast
    When  a user "Frank" joins the session with ID "QUIZ-ABC"
    Then  "Frank" is added to the session's participant list
    And   "Frank"'s initial score is 0
    And   "Frank" receives the current question
    And   "Frank" can submit answers from the current question onward

  # ----- Error Cases -----

  Scenario: Joining a non-existent quiz session
    Given no quiz session exists with ID "QUIZ-999"
    When  a user "Alice" tries to join session "QUIZ-999"
    Then  the system returns an error "Quiz session not found"
    And   "Alice" is not added to any session

  Scenario: Joining an already completed quiz session
    Given a quiz session exists with ID "QUIZ-OLD"
    And   the session status is "completed"
    When  a user "Alice" tries to join session "QUIZ-OLD"
    Then  the system returns an error "Quiz session has already ended"
    And   "Alice" is not added to the session

  Scenario: Joining with an empty quiz ID
    When  a user "Alice" tries to join session ""
    Then  the system returns an error "Quiz ID is required"
    And   "Alice" is not added to any session
```

---

## Feature: Real-Time Quiz Participation

```gherkin
Feature: Real-Time Quiz Participation
  In order to compete fairly in real time
  As a participant in a quiz session
  I want to receive questions and submit answers as the quiz runs

  Background:
    Given a quiz session "QUIZ-ABC" is active
    And   "Alice" is a participant in session "QUIZ-ABC"

  # ----- Happy Path -----

  Scenario: Submitting a correct answer
    Given the current question is:
      """
      {
        "id": "Q1",
        "text": "What is the past tense of 'go'?",
        "options": ["goed", "went", "gone", "going"],
        "correctAnswer": "went"
      }
      """
    When  "Alice" submits answer "went" for question "Q1"
    Then  the answer is marked as correct
    And   "Alice"'s score is increased by the base points

  Scenario: Submitting an incorrect answer
    Given the current question has correct answer "went" for question "Q1"
    When  "Alice" submits answer "goed" for question "Q1"
    Then  the answer is marked as incorrect
    And   "Alice"'s score is not increased

  Scenario: Broadcasting a question to all participants
    Given the following participants are connected:
      """
      Alice
      Bob
      Charlie
      """
    When  the next question is broadcast
    Then  all 3 participants receive the question simultaneously
    And   each participant can submit an answer

  # ----- Edge Cases -----

  Scenario: Preventing duplicate answer submissions
    Given "Alice" has already submitted answer "went" for question "Q1"
    When  "Alice" submits another answer "gone" for question "Q1"
    Then  the system rejects the duplicate submission
    And   "Alice"'s score remains unchanged
    And   the system returns an error "Answer already submitted for this question"

  Scenario: Submitting an answer for a non-existent question
    When  "Alice" submits answer "went" for question "Q-NONEXISTENT"
    Then  the system returns an error "Question not found"
    And   "Alice"'s score remains unchanged
```

---

## Feature: Real-Time Score Updates

```gherkin
Feature: Real-Time Score Updates
  In order to know where I stand instantly
  As a participant in a quiz session
  I want my score updated and broadcast the moment I answer

  Background:
    Given a quiz session "QUIZ-ABC" is active
    And   the base points per correct answer is 10

  # ----- Happy Path -----

  Scenario: Score is updated immediately upon correct answer
    Given "Alice" is a participant with score 10
    When  "Alice" submits a correct answer
    Then  "Alice"'s score becomes 20
    And   the score update is broadcast to all participants within 500ms

  Scenario: Score broadcast reaches all participants
    Given the following participants are connected:
      """
      Alice (score: 10)
      Bob (score: 20)
      Charlie (score: 15)
      """
    When  "Alice" submits a correct answer and her score becomes 20
    Then  "Bob" receives the score update for "Alice"
    And   "Charlie" receives the score update for "Alice"
    And   the update is received within 500ms

  # ----- Consistency -----

  Scenario: Scoring is accurate — identical inputs produce identical scores
    Given "Alice" and "Bob" both have score 0
    When  "Alice" submits a correct answer for question "Q1"
    And   "Bob" submits a correct answer for question "Q1"
    Then  both "Alice" and "Bob" have score 10

  Scenario: Scores do not change retroactively
    Given "Alice" has score 30 after answering 3 questions correctly
    When  a new question "Q4" is broadcast
    Then  "Alice"'s score remains 30 until she submits an answer for "Q4"

  Scenario: Score is not increased for incorrect answer
    Given "Alice" is a participant with score 10
    When  "Alice" submits an incorrect answer
    Then  "Alice"'s score remains 10
```

---

## Feature: Real-Time Leaderboard

```gherkin
Feature: Real-Time Leaderboard
  In order to feel the thrill of competition
  As a participant in a quiz session
  I want a live ranked leaderboard that updates the instant any score changes

  Background:
    Given a quiz session "QUIZ-ABC" is active

  # ----- Happy Path -----

  Scenario: Leaderboard displays correct rankings
    Given the participants have the following scores:
      """
      Alice: 30
      Bob: 50
      Charlie: 40
      """
    When  the leaderboard is calculated
    Then  the leaderboard displays:
      """
      1. Bob — 50
      2. Charlie — 40
      3. Alice — 30
      """

  Scenario: Leaderboard updates when a score changes
    Given the leaderboard is:
      """
      1. Bob — 50
      2. Charlie — 40
      3. Alice — 30
      """
    When  "Alice" submits a correct answer and her score becomes 60
    Then  the leaderboard updates to:
      """
      1. Alice — 60
      2. Bob — 50
      3. Charlie — 40
      """
    And   all participants receive the updated leaderboard within 500ms

  Scenario: Leaderboard is broadcast to all connected participants
    Given 10 participants are connected
    When  any participant's score changes
    Then  all 10 participants receive the updated leaderboard

  # ----- Tie-Breaking -----

  Scenario: Tie-breaking by earlier submission time
    Given "Alice" reached score 40 at timestamp T1
    And   "Bob" reached score 40 at timestamp T2
    And   T1 is before T2
    When  the leaderboard is calculated
    Then  "Alice" is ranked above "Bob"
    And   the ranking is deterministic

  Scenario: Multiple participants tied at same score
    Given the participants have the following scores:
      """
      Alice: 40 (reached at T1)
      Bob: 40 (reached at T2)
      Charlie: 40 (reached at T3)
      """
    And   T1 < T2 < T3
    When  the leaderboard is calculated
    Then  the leaderboard displays:
      """
      1. Alice — 40
      2. Bob — 40
      3. Charlie — 40
      """
```

---

## Feature: Connection Management & Reliability

```gherkin
Feature: Connection Management and Reliability
  In order to not lose my progress when my network hiccups
  As a participant in a quiz session
  I want my state preserved across disconnects and concurrent activity

  Background:
    Given a quiz session "QUIZ-ABC" is active

  # ----- Disconnection Handling -----

  Scenario: Preserving state on client disconnection
    Given "Alice" is a participant with score 30
    When  "Alice"'s network connection drops
    Then  "Alice"'s score of 30 is preserved in the session
    And   the leaderboard continues to show "Alice" with score 30

  Scenario: Reconnecting after disconnection
    Given "Alice" was a participant with score 30
    And   "Alice"'s connection was dropped
    When  "Alice" reconnects to session "QUIZ-ABC"
    Then  "Alice" resumes with her previous score of 30
    And   "Alice" can continue submitting answers

  # ----- Concurrent Access -----

  Scenario: Handling simultaneous answer submissions without race conditions
    Given 100 participants are connected
    When  all 100 participants submit answers for question "Q1" simultaneously
    Then  all scores are calculated correctly
    And   no scores are lost or duplicated
    And   the leaderboard reflects accurate rankings

  Scenario: No data corruption under concurrent writes
    Given "Alice" submits an answer at the exact same time as "Bob"
    When  both answers are processed
    Then  "Alice"'s score reflects only her answer
    And   "Bob"'s score reflects only his answer
    And   neither score is corrupted
```

---

## Feature: Session Lifecycle

```gherkin
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
  # An absent (AFK) or disconnected participant can never block the quiz; they
  # are simply scored 0 for questions they miss.

  # ----- Configuring the End Policy at Creation -----

  Scenario: Creating a session with the manual end policy
    Given the quiz server is running
    When  a host creates a new quiz session with end policy "manual"
    Then  the session is created with end policy "manual"
    And   the session status is "waiting"

  Scenario: Creating a session with the timed end policy
    Given the quiz server is running
    When  a host creates a new quiz session with end policy "timed" and a per-question time limit of 30 seconds
    Then  the session is created with end policy "timed"
    And   each question carries a time limit of 30 seconds

  Scenario: Rejecting a timed session with a non-positive time limit
    Given the quiz server is running
    When  a host creates a new quiz session with end policy "timed" and a per-question time limit of 0 seconds
    Then  the system returns an error "Time limit must be greater than zero"
    And   no session is created

  # ----- Quiz Start (common to both policies) -----

  Scenario: Starting a quiz session
    Given a quiz session "QUIZ-ABC" exists with status "waiting"
    And   5 participants have joined
    When  the host starts the quiz
    Then  the session status changes to "active"
    And   the first question is broadcast to all participants

  # ----- Manual Policy: Host-Controlled Advancement & End -----

  Scenario: Host advances to the next question
    Given a quiz session "QUIZ-ABC" is active with end policy "manual"
    And   the current question is "Q1"
    When  the host advances to the next question
    Then  question "Q2" is broadcast to all participants
    And   no further answers are accepted for "Q1"

  Scenario: Host ends the quiz while a participant is AFK
    Given a quiz session "QUIZ-ABC" is active with end policy "manual"
    And   the participants are:
      """
      Alice   (answered)
      Bob     (answered)
      Charlie (AFK — never answered)
      """
    When  the host ends the quiz
    Then  the session status changes to "completed"
    And   the final leaderboard is displayed to all participants
    And   "Charlie" is scored 0 for the unanswered question
    And   no further answer submissions are accepted

  Scenario: Quiz completes after the host advances past the final question
    Given a quiz session "QUIZ-ABC" is active with end policy "manual"
    And   the current question is the final question "Q5"
    When  the host advances past "Q5"
    Then  the session status changes to "completed"
    And   the final leaderboard is displayed to all participants

  # ----- Timed Policy: Automatic Advancement & End -----

  Scenario: Question auto-advances when its time limit expires
    Given a quiz session "QUIZ-ABC" is active with end policy "timed" and a 30 second time limit
    And   the current question "Q1" has been open for 30 seconds
    And   "Charlie" has not answered "Q1"
    When  the time limit for "Q1" expires
    Then  question "Q2" is broadcast to all participants
    And   "Charlie" is scored 0 for "Q1"
    And   no further answers are accepted for "Q1"

  Scenario: Question advances early when all active participants have answered
    Given a quiz session "QUIZ-ABC" is active with end policy "timed" and a 30 second time limit
    And   3 participants are connected
    And   the current question is "Q1"
    When  all 3 connected participants submit an answer for "Q1" before the time limit
    Then  question "Q2" is broadcast without waiting for the timer to expire

  Scenario: An AFK participant never blocks a timed quiz from ending
    Given a quiz session "QUIZ-ABC" is active with end policy "timed" and a 30 second time limit
    And   the quiz has 5 questions total
    And   "Charlie" goes AFK after question 1
    When  the time limit expires for each remaining question
    Then  the quiz reaches the final question without waiting for "Charlie"
    And   after the final question's time limit expires the session status changes to "completed"
    And   the final leaderboard is displayed to all participants

  Scenario: Rejecting a late answer after a question's time limit expired
    Given a quiz session "QUIZ-ABC" is active with end policy "timed" and a 30 second time limit
    And   the time limit for question "Q1" has expired
    When  "Alice" submits an answer for "Q1"
    Then  the system returns an error "Time is up for this question"
    And   "Alice"'s score remains unchanged
    And   the quiz continues to the next question

  # ----- End Guards (common to both policies) -----

  Scenario: Rejecting answer submissions after the quiz ends
    Given a quiz session "QUIZ-ABC" has status "completed"
    And   "Alice" is a participant
    When  "Alice" tries to submit an answer
    Then  the system returns an error "Quiz has already ended"
    And   "Alice"'s score remains unchanged

  Scenario: Ending an already-completed quiz is rejected
    Given a quiz session "QUIZ-ABC" has status "completed"
    When  the host ends the quiz again
    Then  the system returns an error "Quiz has already ended"
    And   the session remains "completed"
```

---

## Feature: Performance Under Load

```gherkin
Feature: Performance Under Load
  In order to keep the experience real-time at scale
  As the platform operator
  I want latency and throughput targets met under heavy load

  Scenario: Maintaining latency under normal load
    Given a quiz session "QUIZ-ABC" is active
    And   100 participants are connected
    When  participants submit answers at a rate of 10 per second
    Then  all score updates are broadcast within 500ms at p95
    And   the leaderboard updates within 500ms at p95

  Scenario: Server handles high message throughput
    Given the quiz server is running
    And   50 concurrent quiz sessions are active
    And   each session has 20 participants
    When  all participants submit answers simultaneously
    Then  the server processes all messages without dropping any
    And   no WebSocket connections are terminated unexpectedly
```

---

## Feature: Input Validation

```gherkin
Feature: Input Validation
  In order to keep scoring trustworthy
  As the platform operator
  I want malformed or empty answers rejected with clear errors

  Background:
    Given a quiz session "QUIZ-ABC" is active
    And   "Alice" is a participant

  Scenario: Rejecting empty answer submission
    When  "Alice" submits an empty answer "" for question "Q1"
    Then  the system returns an error "Answer cannot be empty"

  Scenario: Rejecting answer with invalid format
    When  "Alice" submits an answer that is not one of the valid options
    Then  the system returns an error "Invalid answer option"
```

---

## Test Coverage Summary

| Feature Area              | Scenarios | Priority |
|---------------------------|-----------|----------|
| Quiz Session Management   | 8         | P1       |
| Quiz Participation        | 5         | P1       |
| Score Updates             | 5         | P1       |
| Leaderboard               | 5         | P1       |
| Connection & Reliability  | 4         | P1       |
| Session Lifecycle         | 13        | P1       |
| Performance Under Load    | 2         | P2       |
| Input Validation          | 2         | P2       |
| **Total**                 | **44**    |          |

### Requirements Traceability

| Requirement (PRD)                        | Covered By                                                |
|------------------------------------------|-----------------------------------------------------------|
| FR-1.1 / AC-5 Session creation           | "Creating a new quiz session", "Starting a quiz session"  |
| FR-1.2 / FR-1.3 Join (single + multi)    | "Joining… valid ID", "Multiple users… simultaneously"     |
| FR-1.4 Reject invalid joins              | "Joining a non-existent / completed / empty quiz session" |
| FR-2.5 Late-joining users                | "Joining a quiz session already in progress (late join)"  |
| FR-2.2 / FR-2.3 Answer + validation      | "Submitting a correct / incorrect answer"                 |
| FR-2.4 Duplicate prevention              | "Preventing duplicate answer submissions"                 |
| FR-3.1–3.5 Score calc + broadcast        | All "Real-Time Score Updates" scenarios                   |
| FR-4.1–4.5 Leaderboard ranking/broadcast | "Leaderboard displays / updates / is broadcast"           |
| FR-4.6 Tie-breaking (LastScoredAt)       | "Tie-breaking by earlier submission time"                 |
| NFR-3.1 / NFR-3.2 Reliability            | "Preserving state…", "Reconnecting…"                      |
| NFR-1.1 / NFR-2.1 Scale + latency        | All "Performance Under Load" scenarios                    |
| **FR-5 End policy (manual/timed) — NEW** ⚠️ | All "Session Lifecycle" config/advancement/end scenarios |
| **FR-5 AFK never blocks progress — NEW** ⚠️ | "Host ends… while AFK", "AFK participant never blocks…"   |

> ⚠️ **Proposed requirement — needs PRD/architecture sync.** The end-policy and
> per-question-timer behavior (`manual` vs `timed`) is **new** and not yet in the PRD.
> Before implementation, add: a functional requirement (suggested **FR-5: Quiz End Policy**),
> `EndPolicy` + `TimeLimit` fields on `QuizSession`, and a "Quiz Timer / Advancement"
> responsibility in the architecture (Session Lifecycle owner + per-question `time.AfterFunc`).
> These scenarios are the source of truth for that work.

> **AI Collaboration:** Scenarios generated with Claude (testing-qa + testing-patterns
> skills), reviewed for behavior-focus (no implementation-detail assertions), reformatted
> to canonical Gherkin (documentation skill). Verification: every PRD acceptance criterion
> is mapped above; the late-join scenario closed the FR-2.5 gap; the Session Lifecycle
> feature was rewritten to remove the liveness bug where one AFK participant could block
> quiz completion indefinitely — replaced with `manual`/`timed` end policies that depend
> only on active participants and/or the clock.
