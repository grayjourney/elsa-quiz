# Product Requirements Document (PRD)
# Real-Time Vocabulary Quiz — ELSA English Learning Platform

**Version:** 1.0  
**Date:** 2026-06-13  
**Status:** Draft  
**Author:** Business Analysis (AI-Assisted)

---

## 1. Executive Summary

The Real-Time Vocabulary Quiz is a competitive, multiplayer quiz feature for an English-learning application. Users join quiz sessions via a unique ID, answer vocabulary questions in real time, and compete on a live leaderboard. The feature is designed to increase learner engagement, retention, and motivation through social competition and instant feedback.

---

## 2. Business Objectives

| Objective | KPI | Target |
|-----------|-----|--------|
| Increase daily active engagement | Session participation rate | ≥ 30% of DAU within 3 months |
| Improve vocabulary retention | Post-quiz retention score | ≥ 15% lift vs. solo study |
| Drive social virality | Shared quiz invites per user | ≥ 2 invites/week |
| Reduce churn | 7-day retention | ≥ 5% improvement |

---

## 3. User Personas

| Persona | Description | Primary Need |
|---------|-------------|--------------|
| **Learner (Participant)** | English language student using ELSA app | Join quizzes, answer questions, see scores in real time |
| **Quiz Host** | Teacher or content creator | Create a quiz session (minimal role in MVP: session creation + start/end only) |
| **Observer** | Non-participant viewer | Watch the leaderboard and quiz progress live |

> **Scope note:** The **Observer** persona is a *future* persona only — no functional requirement in this MVP supports a read-only viewer (see §4.2). It is listed to inform forward-looking architecture decisions (e.g., the leaderboard broadcast model can later fan out to non-participants without rework). The **Quiz Host** is intentionally minimal in the MVP: it exists to satisfy "create a quiz session" but content authoring and host controls are out of scope.

---

## 4. Feature Scope

### 4.1 In Scope

- Real-time quiz session creation and joining via unique quiz ID
- Multi-user simultaneous participation in a single quiz session
- Real-time answer submission and score calculation
- Real-time leaderboard with live ranking updates
- Server-side core component handling connections, scoring, and leaderboard

### 4.2 Out of Scope (Future Phases)

- Quiz content management / authoring tool
- User authentication and profile management (assume pre-authenticated users)
- Payment or subscription integration
- Mobile-native client implementations (web-first MVP)
- Historical analytics and reporting dashboards
- Social features (chat, reactions)

---

## 5. Functional Requirements

### FR-1: Quiz Session Management

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-1.1 | System shall allow creation of a quiz session with a system-generated unique quiz ID | Must Have |
| FR-1.2 | System shall allow a user to join an active quiz session by providing the quiz ID | Must Have |
| FR-1.3 | System shall support multiple users (≥100) joining the same quiz session simultaneously | Must Have |
| FR-1.4 | System shall reject join attempts for non-existent or expired quiz IDs with a clear error message | Must Have |
| FR-1.5 | System shall track all connected participants in a session | Must Have |

### FR-2: Real-Time Quiz Participation

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-2.1 | System shall deliver quiz questions to all participants simultaneously | Must Have |
| FR-2.2 | System shall accept answer submissions from participants in real time | Must Have |
| FR-2.3 | System shall validate submitted answers against the correct answer | Must Have |
| FR-2.4 | System shall prevent duplicate answer submissions for the same question by the same user | Should Have |
| FR-2.5 | System shall handle late-joining users gracefully (join mid-quiz) | Should Have |

### FR-3: Real-Time Score Updates

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-3.1 | System shall calculate the user's score immediately upon answer submission | Must Have |
| FR-3.2 | System shall award points based on correctness of the answer | Must Have |
| FR-3.3 | Scoring system must be accurate: identical inputs must produce identical scores | Must Have |
| FR-3.4 | Scoring system must be consistent: scores must not change retroactively | Must Have |
| FR-3.5 | System shall broadcast updated scores to all participants in real time (< 500ms latency) | Must Have |
| FR-3.6 | System may support bonus points for speed of correct answers | Nice to Have |

### FR-4: Real-Time Leaderboard

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-4.1 | System shall maintain a ranked leaderboard of all participants in a quiz session | Must Have |
| FR-4.2 | Leaderboard shall update promptly as scores change (< 500ms after score update) | Must Have |
| FR-4.3 | Leaderboard shall display participant identifier and current score | Must Have |
| FR-4.4 | Leaderboard shall display rank position for each participant | Must Have |
| FR-4.5 | System shall broadcast leaderboard updates to all connected clients in real time | Must Have |
| FR-4.6 | System shall handle tie-breaking consistently (e.g., earlier submission wins) | Should Have |

---

## 6. Non-Functional Requirements

### NFR-1: Scalability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-1.1 | System shall support at least 100 concurrent users per quiz session | Must Have |
| NFR-1.2 | System shall support at least 1,000 concurrent quiz sessions | Should Have |
| NFR-1.3 | Architecture shall allow horizontal scaling of the quiz server | Must Have |

### NFR-2: Performance

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-2.1 | End-to-end latency for score update broadcast shall be < 500ms at p95 | Must Have |
| NFR-2.2 | Leaderboard calculation shall complete in < 100ms for 100 participants | Must Have |
| NFR-2.3 | WebSocket message throughput shall handle ≥ 1,000 messages/sec per server | Should Have |

### NFR-3: Reliability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-3.1 | System shall handle client disconnections gracefully (auto-reconnect support) | Must Have |
| NFR-3.2 | System shall not lose score data on transient failures | Must Have |
| NFR-3.3 | System shall return meaningful error messages for all failure modes | Must Have |
| NFR-3.4 | System uptime target: 99.9% | Should Have |

### NFR-4: Maintainability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-4.1 | Code shall follow clean architecture principles with clear separation of concerns | Must Have |
| NFR-4.2 | All public interfaces shall be documented | Must Have |
| NFR-4.3 | Code shall have ≥ 80% unit test coverage | Must Have |
| NFR-4.4 | Code shall pass linter checks with zero warnings | Should Have |

### NFR-5: Monitoring & Observability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-5.1 | System shall expose health check endpoints | Must Have |
| NFR-5.2 | System shall emit structured logs for all significant events | Must Have |
| NFR-5.3 | System shall expose metrics (connections, messages, latency, errors) | Must Have |
| NFR-5.4 | System shall support distributed tracing | Should Have |

---

## 7. Acceptance Criteria (Business)

### AC-1: User Participation

```
GIVEN a quiz session exists with ID "QUIZ-123"
WHEN a user submits the quiz ID "QUIZ-123" to join
THEN the user is added to the session's participant list
AND the user receives confirmation of successful join
AND all other participants are notified of the new participant
```

```
GIVEN a quiz session exists with ID "QUIZ-123"
AND 50 users are already connected
WHEN a 51st user submits the quiz ID "QUIZ-123" to join
THEN the 51st user is added to the session successfully
AND all 51 users can participate simultaneously
```

```
GIVEN no quiz session exists with ID "QUIZ-999"
WHEN a user submits the quiz ID "QUIZ-999" to join
THEN the user receives an error: "Quiz session not found"
AND the user is not added to any session
```

### AC-2: Real-Time Score Updates

```
GIVEN a user "Alice" is a participant in quiz session "QUIZ-123"
AND the current question has correct answer "B"
WHEN "Alice" submits answer "B"
THEN "Alice"'s score is increased by the correct number of points
AND the updated score is broadcast to all participants within 500ms
```

```
GIVEN a user "Alice" is a participant in quiz session "QUIZ-123"
AND the current question has correct answer "B"
WHEN "Alice" submits answer "A"
THEN "Alice"'s score is NOT increased
AND the updated state is broadcast to all participants within 500ms
```

```
GIVEN a user "Alice" has already submitted an answer for question 3
WHEN "Alice" submits another answer for question 3
THEN the system rejects the duplicate submission
AND "Alice"'s score remains unchanged
```

### AC-3: Real-Time Leaderboard

```
GIVEN quiz session "QUIZ-123" has participants:
  - "Alice" with score 30
  - "Bob" with score 50
  - "Charlie" with score 40
WHEN the leaderboard is requested or a score changes
THEN the leaderboard displays:
  1. Bob — 50 points
  2. Charlie — 40 points
  3. Alice — 30 points
AND the leaderboard is broadcast to all participants in real time
```

```
GIVEN "Alice" and "Bob" both have score 40
WHEN the leaderboard is calculated
THEN the participant who reached 40 first is ranked higher
AND ranking is deterministic and consistent
```

### AC-4: Error Handling & Reliability

```
GIVEN a user "Alice" is connected to quiz session "QUIZ-123"
WHEN "Alice"'s network connection drops
THEN the system preserves "Alice"'s score and state
AND "Alice" can reconnect and resume participation
AND the leaderboard continues to show "Alice"'s last known score
```

```
GIVEN the quiz server is under load with 100 concurrent users
WHEN all users submit answers simultaneously
THEN all scores are calculated correctly (no race conditions)
AND the leaderboard reflects the accurate rankings
AND end-to-end latency remains under 500ms at p95
```

### AC-5: Session Lifecycle

```
GIVEN a quiz host creates a new quiz session
WHEN the session is created
THEN the system generates a unique quiz ID
AND the session is available for users to join
```

```
GIVEN a quiz session "QUIZ-123" has completed all questions
WHEN the quiz ends
THEN the final leaderboard is displayed to all participants
AND no further answer submissions are accepted
AND the session is marked as completed
```

---

## 8. Data Requirements

### 8.1 Core Entities

| Entity | Key Attributes |
|--------|---------------|
| **QuizSession** | ID (unique), Status (waiting/active/completed), Questions[], CreatedAt |
| **Participant** | UserID, SessionID, DisplayName, Score, **LastScoredAt**, JoinedAt |
| **Question** | ID, QuestionText, Options[], CorrectAnswer, Order |
| **Answer** | ParticipantID, QuestionID, SelectedAnswer, IsCorrect, SubmittedAt |
| **LeaderboardEntry** | SessionID, ParticipantID, Score, Rank, UpdatedAt |

> **`LastScoredAt`** is the timestamp at which a participant *reached their current score*. It is required to make tie-breaking (FR-4.6) deterministic: when two participants share a score, the one who reached it earlier ranks higher. Without it, ranking among equal scores is non-deterministic.

### 8.2 Data Flow Summary

```
User Joins → WebSocket Connection → Add to Session
  → Question Broadcast → Answer Submission
  → Score Calculation → Score Broadcast
  → Leaderboard Recalculation → Leaderboard Broadcast
```

---

## 9. Constraints & Assumptions

### Constraints
- Implementation focuses on **one core server component** (server handling connections, scoring, leaderboard)
- Remaining system components (client apps, database persistence, auth) are **mocked**
- Technology choice: **Go (Golang)** for the server component
- Must use **WebSocket** for real-time communication

### Assumptions
- Users are pre-authenticated; user identity is provided at connection time
- Quiz content (questions) is pre-loaded; content authoring is out of scope
- Network latency between client and server is < 100ms in normal conditions
- A single server instance should handle the MVP load; horizontal scaling is a design consideration

---

## 10. Success Metrics

| Metric | Measurement | Target |
|--------|-------------|--------|
| Functional correctness | All acceptance criteria pass | 100% |
| Test coverage | Unit test coverage | ≥ 80% |
| Latency | Score update end-to-end p95 | < 500ms |
| Concurrency | Simultaneous users per session | ≥ 100 |
| Code quality | Linter warnings | 0 |
| Reliability | Graceful error handling | All failure modes covered |

---

## 11. AI Collaboration Documentation

This PRD was generated with significant AI assistance:
- **Tool**: Gemini / Claude (Business Analyst skill)
- **Task**: Transform raw coding challenge requirements into a structured PRD with quantified acceptance criteria
- **Verification**: All acceptance criteria traced back to original `test.md` requirements; business KPIs are illustrative and should be validated with product stakeholders
