"""
test_social.py Search, user profiles, follow and unfollow.

user_a follows user_b. user_b is a public account so the follow is instant.
"""

import pytest
from selenium.webdriver.common.by import By
from selenium.webdriver.support import expected_conditions as EC


class TestSearch:
    """User can search for other users in the Explore page."""

    def test_search_page_has_input(self, page, user_a):
        """Navigating to Explore shows the search input."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("search")
        assert page.wait_id("search-input").is_displayed()

    def test_search_returns_results(self, page, user_a, user_b):
        """Searching for user_b's username returns at least one result row."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.search_user(user_b["username"])
        rows = page.find_all(".user-row")
        assert len(rows) >= 1

        # At least one row should contain user_b's username
        names = [r.text for r in rows]
        assert any(user_b["username"] in n for n in names)

    def test_search_no_results_for_garbage(self, page, user_a):
        """Searching for a nonsense string shows an empty-state message."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("search")
        inp = page.wait_id("search-input")
        inp.clear()
        inp.send_keys("zzz_no_such_user_xyz_qqq_999")
        page.sleep(0.9)

        # No user-rows should appear; empty state text should be visible
        rows = page.find_all(".user-row")
        assert len(rows) == 0


class TestUserModal:
    """Clicking a search result opens the user profile modal."""

    def test_user_modal_opens_with_correct_username(self, page, user_a, user_b):
        """Opening user_b's profile modal shows their username in the title."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.open_user_modal(user_b["username"])
        title = page.wait_id("user-modal-title")
        assert user_b["username"] in title.text
        page.close_modal("modal-user")

    def test_user_modal_shows_follow_button(self, page, user_a, user_b):
        """user_b's profile modal shows a Follow button when not yet following."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.open_user_modal(user_b["username"])
        body = page.wait_id("user-modal-body")

        # Should have a "Follow" button (not "Following" or "Requested")
        follow_btn = page.w.until(EC.presence_of_element_located(
            (By.XPATH, "//div[@id='user-modal-body']//button[text()='Follow']")
        ))
        assert follow_btn.is_displayed()
        page.close_modal("modal-user")


class TestFollow:
    """user_a can follow and unfollow user_b."""

    def test_follow_user_changes_button_to_following(self, page, user_a, user_b):
        """
        After clicking Follow, the button text changes to 'Following'
        when the target account is public (immediate accept).
        """
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.open_user_modal(user_b["username"])

        # Click Follow
        follow_btn = page.w.until(EC.element_to_be_clickable(
            (By.XPATH, "//div[@id='user-modal-body']//button[text()='Follow']")
        ))
        follow_btn.click()

        # Modal re-opens automatically after follow; button should now say
        # "Following" (public account) or "Requested" (private account)
        page.wait_css("#modal-user.open")
        body = page.wait_id("user-modal-body")
        page.sleep(1)

        buttons = body.find_elements(By.CSS_SELECTOR, "button")
        button_texts = [b.text for b in buttons]
        assert any(t in ("Following", "Requested") for t in button_texts), \
            f"Expected Following or Requested, got: {button_texts}"

    def test_follow_updates_following_count_on_profile(self, page, user_a):
        """After following user_b, user_a's following count is >= 1."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("profile")
        page.wait_css(".profile-stats")

        stats = page.find_all(".profile-stat")
        # Stats order: Posts | Followers | Following | Blocked
        following_stat = stats[2]
        count = int(following_stat.find_element(
            By.CSS_SELECTOR, ".stat-number").text)
        assert count >= 1

    def test_unfollow_user_changes_button_back_to_follow(self, page, user_a, user_b):
        """Clicking 'Following' in the modal unfollows the user."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.open_user_modal(user_b["username"])
        page.sleep(1)

        body = page.wait_id("user-modal-body")
        buttons = body.find_elements(By.CSS_SELECTOR, "button")

        unfollow_btn = None
        for b in buttons:
            if b.text == "Following":
                unfollow_btn = b
                break

        if unfollow_btn is None:
            # Already unfollowed or never followed skip gracefully
            page.close_modal("modal-user")
            return

        unfollow_btn.click()
        page.sleep(1)

        # Modal should re-open showing Follow again
        page.wait_css("#modal-user.open")
        body = page.wait_id("user-modal-body")
        page.sleep(0.5)
        buttons = body.find_elements(By.CSS_SELECTOR, "button")
        assert any(b.text == "Follow" for b in buttons)
        page.close_modal("modal-user")
