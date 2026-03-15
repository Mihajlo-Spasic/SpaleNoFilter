# Lumina = Selenium UI Tests

End-to-end browser tests for the Lumina frontend.
Chrome (headless) is driven via Selenium 4 + WebDriver Manager.

## Prerequisites

- Python 3.10+
- Google Chrome installed
- The full docker-compose stack running (`docker compose up -d`)
- The frontend served at `http://localhost:3000` (nginx) **or** opened
  directly, in which case set `FRONTEND_URL` (see below)

## Setup

```bash
cd ui-tests
python -m venv .venv
source .venv/bin/activate          # Windows: .venv\Scripts\activate
pip install -r requirements.txt
```

## Running the tests

```bash
# All tests (recommended — runs in correct order)
pytest

# Specific file
pytest test_auth.py -v

# Specific test class
pytest test_post.py::TestLikes -v

# Stop on first failure
pytest -x
```

An HTML report is written to `report.html` after every run.

## Environment variables

| Variable           | Default                  | Description                  |
| ------------------ | ------------------------ | ---------------------------- |
| `FRONTEND_URL`     | `http://localhost:13000` | URL of the Lumina SPA        |
| `SELENIUM_TIMEOUT` | `15`                     | Seconds to wait for elements |

Example — if you open the HTML file directly from disk:

```bash
FRONTEND_URL="file:///home/user/bazeFinals/lumina/index.html" pytest
```

## Test coverage

| File              | What is tested                                             |
| ----------------- | ---------------------------------------------------------- |
| `test_auth.py`    | Register (success, duplicate username, duplicate email)    |
|                   | Login (username, email, wrong password, non-existent user) |
|                   | Logout (returns to auth screen, re-login works)            |
| `test_post.py`    | Upload post with caption                                   |
|                   | Post appears in feed and profile grid                      |
|                   | Like / unlike (counter, CSS class)                         |
|                   | Comment section open, add comment, delete comment          |
|                   | Delete post via ••• modal                                  |
| `test_social.py`  | Search returns results / empty state                       |
|                   | User profile modal opens with correct username             |
|                   | Follow button visible; follow changes button to Following  |
|                   | Following count updates on own profile                     |
|                   | Unfollow changes button back to Follow                     |
| `test_profile.py` | Profile page shows username, stats, post grid              |
|                   | Sidebar shows correct username                             |
|                   | Settings pre-filled with current values                    |
|                   | Save bio → toast → bio visible on profile                  |
|                   | Wrong password shows error                                 |
|                   | Delete account → auth screen → login fails                 |
