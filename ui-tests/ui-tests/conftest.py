"""
conftest.py Shared pytest fixtures for Lumina UI tests.

Provides:
  - A headless Chrome WebDriver (one per test session for speed)
  - A LuminaPage helper that wraps common UI interactions
  - Two pre-registered test users (user_a and user_b) available to all tests
"""

import os
import time
import uuid
import pytest

from selenium import webdriver
from selenium.webdriver.firefox.options import Options
from selenium.webdriver.firefox.service import Service
from selenium.webdriver.common.by import By
from selenium.webdriver.common.keys import Keys
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.common.exceptions import TimeoutException, NoSuchElementException
from webdriver_manager.firefox import GeckoDriverManager


BASE_URL = os.getenv("LUMINA_URL", "http://localhost:8080")
# When running the SPA directly from the filesystem or nginx on :3000 change this.
# The tests open the HTML file served from whatever URL is given here.
FRONTEND_URL = os.getenv("FRONTEND_URL", "http://localhost:13000")
TIMEOUT = int(os.getenv("SELENIUM_TIMEOUT", "15"))

# A small real PNG (1×1 red pixel) encoded as bytes — used for photo uploads
# without needing an external file.
TINY_PNG = (
    b"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01"
    b"\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\x0f\x00"
    b"\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82"
)


@pytest.fixture(scope="session")
def driver():
    """Single headless Chrome instance shared across the whole test session."""
    options = Options()
    # options.add_argument("--headless")
    options.add_argument("--width=1400")
    options.add_argument("--height=900")

    service = Service(GeckoDriverManager().install())
    drv = webdriver.Firefox(service=service, options=options)
    drv.implicitly_wait(0)   # we use explicit waits everywhere
    yield drv
    drv.quit()


class LuminaPage:
    """
    Thin wrapper around WebDriver that provides named helpers for every
    interaction the UI tests need. All waits are explicit with a shared timeout.
    """

    def __init__(self, driver):
        self.d = driver
        self.w = WebDriverWait(driver, TIMEOUT)

    def wait_id(self, element_id, visible=True):
        """Wait for an element by ID, optionally requiring it to be visible."""
        condition = (
            EC.visibility_of_element_located((By.ID, element_id))
            if visible
            else EC.presence_of_element_located((By.ID, element_id))
        )
        return self.w.until(condition)

    def wait_css(self, selector, visible=True):
        condition = (
            EC.visibility_of_element_located((By.CSS_SELECTOR, selector))
            if visible
            else EC.presence_of_element_located((By.CSS_SELECTOR, selector))
        )
        return self.w.until(condition)

    def wait_xpath(self, xpath):
        return self.w.until(EC.visibility_of_element_located((By.XPATH, xpath)))

    def wait_text(self, element_id, text):
        """Wait until element with given ID contains the given text."""
        return self.w.until(
            EC.text_to_be_present_in_element((By.ID, element_id), text)
        )

    def wait_toast(self, expected_text):
        """Wait for the toast notification to contain expected text."""
        return self.w.until(
            EC.text_to_be_present_in_element((By.ID, "toast"), expected_text)
        )

    def wait_clickable(self, selector):
        return self.w.until(
            EC.element_to_be_clickable((By.CSS_SELECTOR, selector))
        )

    def wait_gone(self, element_id):
        """Wait until an element is no longer visible."""
        return self.w.until(
            EC.invisibility_of_element_located((By.ID, element_id))
        )

    def find(self, css):
        return self.d.find_element(By.CSS_SELECTOR, css)

    def find_all(self, css):
        return self.d.find_elements(By.CSS_SELECTOR, css)

    def js(self, script, *args):
        return self.d.execute_script(script, *args)

    def sleep(self, seconds=0.5):
        time.sleep(seconds)

    def open(self):
        self.d.get(FRONTEND_URL)
        self.d.execute_script("localStorage.clear(); sessionStorage.clear();")
        self.d.get(FRONTEND_URL)
        self.wait_id("auth")

    def nav_to(self, page_name):
        """Click a sidebar nav item by its data-page attribute."""
        # Close any open modal that might block the click
        try:
            overlay = self.d.find_element(
                "css selector", ".modal-overlay.open")
            close_btn = overlay.find_element("css selector", ".modal-close")
            close_btn.click()
            import time
            time.sleep(0.3)
        except Exception:
            pass
        item = self.w.until(EC.element_to_be_clickable(
            (By.CSS_SELECTOR, f'.nav-item[data-page="{page_name}"]')
        ))
        item.click()
        self.w.until(EC.visibility_of_element_located(
            (By.ID, f"page-{page_name}")
        ))

    def switch_to_register(self):
        # Wait for auth screen to be visible first
        self.w.until(EC.visibility_of_element_located((By.ID, "auth")))
        self.w.until(lambda d: len(d.find_elements(
            "css selector", ".auth-tab")) >= 2)
        tabs = self.find_all(".auth-tab")
        tabs[1].click()   # "Create Account"
        self.w.until(EC.visibility_of_element_located(
            (By.ID, "register-form")))

    def switch_to_login(self):
        tabs = self.find_all(".auth-tab")
        tabs[0].click()   # "Sign In"
        self.wait_id("login-form")

    def register(self, username, email, fullname, password):
        self.switch_to_register()
        self.wait_id("reg-username").clear()
        self.find("#reg-username").send_keys(username)
        self.find("#reg-email").send_keys(email)
        self.find("#reg-fullname").send_keys(fullname)
        self.find("#reg-password").send_keys(password)
        self.wait_clickable("#register-form button.btn-primary.full").click()
        # Success: app shell becomes visible
        self.wait_css("#app.visible")

    def login(self, identifier, password):
        self.switch_to_login()
        self.wait_id("login-identifier").clear()
        self.find("#login-identifier").send_keys(identifier)
        self.find("#login-password").clear()
        self.find("#login-password").send_keys(password)
        self.wait_clickable("#login-form button.btn-primary.full").click()
        self.wait_css("#app.visible")

    def logout(self):
        """Click the logout button in the sidebar footer."""
        btn = self.w.until(EC.element_to_be_clickable(
            (By.CSS_SELECTOR, ".sidebar-footer .btn-ghost")
        ))
        btn.click()
        self.wait_id("auth")

    def upload_post(self, image_path, caption=""):
        """Open the New Post modal, attach a file, optionally set caption, submit."""
        new_post_btn = self.w.until(EC.element_to_be_clickable(
            (By.XPATH,
             "//div[contains(@class,'nav-item') and not(@data-page)]")
        ))
        new_post_btn.click()
        self.wait_css("#modal-upload.open")

        # Send the file path directly to the hidden file input
        file_input = self.w.until(EC.presence_of_element_located(
            (By.ID, "upload-file-input")
        ))
        self.js("arguments[0].style.display = 'block'", file_input)
        file_input.send_keys(image_path)

        if caption:
            cap = self.wait_id("upload-caption")
            cap.clear()
            cap.send_keys(caption)

        self.wait_clickable("#upload-submit-btn").click()
        # Modal should close after successful upload
        self.w.until(EC.invisibility_of_element_located(
            (By.CSS_SELECTOR, "#modal-upload.open")
        ))

    def first_post_card(self):
        """Return the first post-card element in the feed."""
        return self.w.until(EC.presence_of_element_located(
            (By.CSS_SELECTOR, ".post-card")
        ))

    def like_first_post(self):
        card = self.first_post_card()
        like_btn = card.find_element(By.CSS_SELECTOR, ".action-btn")
        like_btn.click()
        return like_btn

    def toggle_comments_first_post(self):
        card = self.first_post_card()
        comment_btn = card.find_elements(By.CSS_SELECTOR, ".action-btn")[1]
        comment_btn.click()

    def add_comment(self, post_id, text):
        inp = self.wait_id(f"comment-input-{post_id}")
        inp.send_keys(text)
        inp.send_keys(Keys.RETURN)
        self.sleep(1)

    def get_first_post_id(self):
        """Extract the post ID from the comments section ID attribute."""
        section = self.w.until(EC.presence_of_element_located(
            (By.CSS_SELECTOR, '[id^="comments-"]')
        ))
        return section.get_attribute("id").replace("comments-", "")

    def search_user(self, query):
        self.nav_to("search")
        inp = self.wait_id("search-input")
        inp.clear()
        inp.send_keys(query)
        # Wait for results to appear (debounce is 320 ms)
        self.sleep(0.8)
        self.wait_css(".user-row")

    def open_user_modal(self, username):
        """Search for a user and click their row to open the profile modal."""
        self.search_user(username)
        row = self.w.until(EC.element_to_be_clickable(
            (By.CSS_SELECTOR, ".user-row")
        ))
        row.click()
        self.wait_css("#modal-user.open")

    def click_follow_in_modal(self):
        btn = self.w.until(EC.element_to_be_clickable(
            (By.XPATH, "//div[@id='user-modal-body']//button[text()='Follow']")
        ))
        btn.click()

    def close_modal(self, modal_id):
        btn = self.w.until(EC.element_to_be_clickable(
            (By.CSS_SELECTOR, f"#{modal_id} .modal-close")
        ))
        btn.click()
        self.w.until(EC.invisibility_of_element_located(
            (By.CSS_SELECTOR, f"#{modal_id}.open")
        ))

    def delete_account(self):
        self.nav_to("settings")
        delete_btn = self.w.until(EC.element_to_be_clickable(
            (By.XPATH,
             "//button[contains(@class,'btn-danger') and text()='Delete']")
        ))
        # Handle the browser confirm() dialog
        self.d.execute_script(
            "window.confirm = function() { return true; }"
        )
        delete_btn.click()
        # After deletion the app logs out and shows the auth screen
        self.wait_id("auth")


@pytest.fixture(scope="session")
def page(driver):
    return LuminaPage(driver)


def make_user():
    """Generate a unique username/email pair for a fresh test user."""
    uid = uuid.uuid4().hex[:8]
    return {
        "username": f"tst{uid}",
        "email":    f"tst{uid}@lumina.test",
        "fullname": f"Test {uid.upper()}",
        "password": f"Pass{uid}1234",
    }


@pytest.fixture(scope="session")
def user_a(page):
    """
    Primary test user registered once, stays logged in for most tests.
    Logged out and re-logged-in where necessary.
    """
    # Clear any stored token so registration starts from auth screen
    page.d.get(FRONTEND_URL)
    page.d.execute_script("localStorage.clear(); sessionStorage.clear();")
    page.d.get(FRONTEND_URL)
    u = make_user()
    page.register(u["username"], u["email"], u["fullname"], u["password"])
    # Stay logged in after registration
    return u


@pytest.fixture(scope="session")
def user_b(page, user_a):
    """
    Secondary test user used as the target for follow/social tests.
    Registered while user_a session is active, then we switch back.
    """
    page.logout()
    u = make_user()
    page.register(u["username"], u["email"], u["fullname"], u["password"])
    page.logout()
    # Return to user_a session
    page.login(user_a["username"], user_a["password"])
    return u


@pytest.fixture(scope="session")
def test_image(tmp_path_factory):
    """Write a tiny valid PNG to a temp file and return its absolute path."""
    p = tmp_path_factory.mktemp("img") / "test.png"
    p.write_bytes(TINY_PNG)
    return str(p)
