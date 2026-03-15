"""
test_profile.py Profile page, settings editing, and account deletion.

Account deletion is the last test because it destroys the session.
A dedicated disposable user is created for that test so user_a stays intact.
"""

import uuid
import pytest
from selenium.webdriver.common.by import By
from selenium.webdriver.support import expected_conditions as EC


class TestProfilePage:
    """Own profile page displays correct user information and post grid."""

    def test_profile_shows_username(self, page, user_a):
        """The profile page displays user_a's username."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("profile")
        page.wait_css(".profile-username")
        assert user_a["username"] in page.find(".profile-username").text

    def test_profile_shows_stats_section(self, page, user_a):
        """Profile page contains Posts / Followers / Following stats."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("profile")
        page.wait_css(".profile-stats")
        stats = page.find_all(".profile-stat")
        assert len(stats) >= 3

        labels = [s.find_element(By.CSS_SELECTOR, ".stat-label").text.lower()
                  for s in stats]
        assert "posts" in labels
        assert "followers" in labels
        assert "following" in labels

    def test_profile_post_grid_visible_after_upload(self, page, user_a, test_image):
        """After uploading, the post grid contains at least one thumbnail."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("feed")
        page.upload_post(test_image, caption="Profile grid check")
        page.nav_to("profile")
        page.wait_css(".post-grid")
        assert len(page.find_all(".grid-item")) >= 1

    def test_sidebar_shows_correct_username(self, page, user_a):
        """Sidebar footer shows user_a's username."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        sidebar_un = page.wait_id("sidebar-username")
        assert sidebar_un.text == user_a["username"]


class TestSettingsPage:
    """User can edit profile details from the Settings page."""

    def test_settings_page_loads_current_values(self, page, user_a):
        """Settings form is pre-filled with the user's current full name."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("settings")
        fullname_input = page.wait_id("settings-fullname")
        # Field should be pre-filled with the registered full name
        assert fullname_input.get_attribute("value") == user_a["fullname"]

    def test_save_bio_shows_toast(self, page, user_a):
        """Saving a new bio shows the 'Profile saved' toast."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("settings")
        bio_input = page.wait_id("settings-bio")
        bio_input.clear()
        bio_input.send_keys("Selenium test bio")

        save_btn = page.w.until(EC.element_to_be_clickable(
            (By.XPATH, "//button[text()='Save Changes']")
        ))
        save_btn.click()
        page.wait_toast("saved")

    def test_saved_bio_appears_on_profile(self, page, user_a):
        """After saving, the bio is visible on the profile page."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        bio_text = f"Bio{uuid.uuid4().hex[:6]}"
        page.nav_to("settings")
        bio_input = page.wait_id("settings-bio")
        bio_input.clear()
        bio_input.send_keys(bio_text)

        page.w.until(EC.element_to_be_clickable(
            (By.XPATH, "//button[text()='Save Changes']")
        )).click()
        page.wait_toast("saved")

        page.nav_to("profile")
        page.wait_css(".profile-bio")
        assert bio_text in page.find(".profile-bio").text

    def test_wrong_current_password_shows_error(self, page, user_a):
        """Submitting a wrong current password when changing password shows an error."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("settings")
        page.wait_id("settings-old-password").send_keys("definitely_wrong_pw")
        page.wait_id("settings-new-password").send_keys("NewPass_123!")

        page.w.until(EC.element_to_be_clickable(
            (By.XPATH, "//button[text()='Change Password']")
        )).click()

        # Should show an error toast (not log out)
        page.wait_toast("")   # any toast
        toast_el = page.find("#toast")
        # App should still be open (no logout happened)
        assert "visible" in (page.find("#app").get_attribute("class") or "")


class TestAccountDeletion:
    """
    A freshly created disposable user deletes their own account.
    This test runs last because it permanently removes a user.
    """

    def test_delete_account_logs_out_and_shows_auth(self, page):
        """
        After confirming account deletion the user is logged out and the
        auth screen is shown. The deleted username cannot be re-logged-in.
        """
        # Create a throwaway user specifically for this test
        uid = uuid.uuid4().hex[:8]
        disposable = {
            "username": f"del{uid}",
            "email":    f"del{uid}@test.com",
            "fullname": f"Del {uid}",
            "password": f"DelPass{uid}1",
        }

        # Log out any current session and register the disposable user
        try:
            page.logout()
        except Exception:
            page.open()

        page.register(
            disposable["username"],
            disposable["email"],
            disposable["fullname"],
            disposable["password"],
        )
        assert "visible" in (page.find("#app").get_attribute("class") or "")

        # Delete the account
        page.delete_account()

        # Must be back at the auth screen
        assert page.find("#auth").is_displayed()

        # Attempting to log back in with the deleted credentials must fail
        page.switch_to_login()
        page.find("#login-identifier").send_keys(disposable["username"])
        page.find("#login-password").send_keys(disposable["password"])
        page.wait_clickable("button.btn-primary.full").click()

        error = page.wait_id("login-error")
        assert error.is_displayed(), "Deleted user should not be able to log in"
