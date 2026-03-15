"""
test_auth.py Registration, login and logout UI tests.

Tests run in order (pytest-ordering not required; they share session state
through the `page` fixture which holds a single browser instance).
"""

import uuid
import pytest
from selenium.webdriver.common.by import By
from selenium.webdriver.support import expected_conditions as EC


class TestRegistration:
    """User can create a new account through the register form."""

    def test_register_form_visible_after_tab_switch(self, page):
        """Clicking 'Create Account' tab shows the registration form."""
        page.open()
        page.switch_to_register()
        assert page.wait_id("register-form").is_displayed()
        assert not page.find("#login-form").is_displayed()

    def test_register_success_shows_app(self, page):
        """Submitting valid details registers the user and shows the app shell."""
        uid = uuid.uuid4().hex[:8]
        page.register(
            username=f"reg{uid}",
            email=f"reg{uid}@test.com",
            fullname=f"Reg {uid}",
            password=f"Password{uid}1",
        )
        # App shell must be visible and auth screen hidden
        assert page.find("#app").get_attribute("class") == "visible" or \
            "visible" in page.find("#app").get_attribute("class")
        assert not page.find("#auth").is_displayed()
        page.logout()

    def test_register_duplicate_username_shows_error(self, page, user_a):
        """Registering with an already-taken username shows an error message."""
        page.open()
        page.switch_to_register()
        uid = uuid.uuid4().hex[:8]
        page.find("#reg-username").send_keys(user_a["username"])  # duplicate
        page.find("#reg-email").send_keys(f"uniq{uid}@test.com")
        page.find("#reg-fullname").send_keys("Duplicate")
        page.find("#reg-password").send_keys(f"Pass{uid}1234")
        page.wait_clickable("#register-form button.btn-primary.full").click()

        error = page.wait_id("register-error")
        assert error.is_displayed()
        assert error.text != ""

    def test_register_duplicate_email_shows_error(self, page, user_a):
        """Registering with an already-taken email shows an error message."""
        page.open()
        page.switch_to_register()
        uid = uuid.uuid4().hex[:8]
        page.find("#reg-username").send_keys(f"uniq{uid}")
        page.find("#reg-email").send_keys(user_a["email"])   # duplicate
        page.find("#reg-fullname").send_keys("Duplicate")
        page.find("#reg-password").send_keys(f"Pass{uid}1234")
        page.wait_clickable("#register-form button.btn-primary.full").click()

        error = page.wait_id("register-error")
        assert error.is_displayed()


class TestLogin:
    """User can sign in with username or email."""

    def test_login_tab_shows_login_form(self, page):
        page.open()
        page.switch_to_login()
        assert page.wait_id("login-form").is_displayed()

    def test_login_with_username_succeeds(self, page, user_a):
        page.open()
        page.login(user_a["username"], user_a["password"])
        assert "visible" in page.find("#app").get_attribute("class")
        # Sidebar should show the username
        sidebar_un = page.wait_id("sidebar-username")
        assert sidebar_un.text == user_a["username"]
        page.logout()

    def test_login_with_email_succeeds(self, page, user_a):
        page.open()
        page.login(user_a["email"], user_a["password"])
        assert "visible" in page.find("#app").get_attribute("class")
        page.logout()

    def test_login_wrong_password_shows_error(self, page, user_a):
        page.open()
        page.switch_to_login()
        page.find("#login-identifier").send_keys(user_a["username"])
        page.find("#login-password").send_keys("wrong_password_xyz")
        page.wait_clickable("button.btn-primary.full").click()

        error = page.wait_id("login-error")
        assert error.is_displayed()
        assert error.text != ""

    def test_login_nonexistent_user_shows_error(self, page):
        page.open()
        page.switch_to_login()
        page.find("#login-identifier").send_keys("no_such_user_xyz_123")
        page.find("#login-password").send_keys("Password_123!")
        page.wait_clickable("button.btn-primary.full").click()

        error = page.wait_id("login-error")
        assert error.is_displayed()


class TestLogout:
    """User can log out and is returned to the auth screen."""

    def test_logout_shows_auth_screen(self, page, user_a):
        page.open()
        page.login(user_a["username"], user_a["password"])
        assert "visible" in page.find("#app").get_attribute("class")

        page.logout()

        assert page.find("#auth").is_displayed()
        assert not page.find("#app").is_displayed() or \
            "visible" not in (page.find("#app").get_attribute("class") or "")

    def test_after_logout_login_works_again(self, page, user_a):
        """Session is fully cleared after logout; re-login must succeed."""
        page.open()
        page.login(user_a["username"], user_a["password"])
        page.logout()
        page.login(user_a["username"], user_a["password"])
        assert "visible" in page.find("#app").get_attribute("class")
