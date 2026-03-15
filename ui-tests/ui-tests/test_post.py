"""
test_post.py Post upload, feed display, likes, comments, and deletion.

Depends on user_a being logged in (ensured by test_auth.py running first,
or by the user_a fixture which leaves user_a logged in on completion).
"""

import pytest
from selenium.webdriver.common.by import By
from selenium.webdriver.support import expected_conditions as EC


class TestPostUpload:
    """User can create a new post with a photo and caption."""

    def test_new_post_modal_opens(self, page, user_a):
        """Clicking 'New Post' in the sidebar opens the upload modal."""
        # Make sure user_a is logged in
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        new_post_nav = page.w.until(EC.element_to_be_clickable(
            (By.XPATH,
             "//div[contains(@class,'nav-item') and not(@data-page)]")
        ))
        new_post_nav.click()
        page.wait_css("#modal-upload.open")
        assert page.find("#modal-upload").is_displayed()
        page.close_modal("modal-upload")

    def test_upload_post_with_caption_succeeds(self, page, user_a, test_image):
        """Uploading a photo with a caption closes the modal and refreshes the feed."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("feed")
        page.upload_post(test_image, caption="Selenium test post 🤖")

        # Modal must be closed
        page.w.until(EC.invisibility_of_element_located(
            (By.CSS_SELECTOR, "#modal-upload.open")
        ))

        # Feed should now contain at least one post card
        page.wait_css(".post-card")
        cards = page.find_all(".post-card")
        assert len(cards) >= 1

    def test_uploaded_post_caption_visible_in_feed(self, page, user_a, test_image):
        """The caption of the uploaded post appears in the feed."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        caption_text = f"Caption_{id(self)}"
        page.nav_to("feed")
        page.upload_post(test_image, caption=caption_text)

        # Find a card that contains the caption
        page.w.until(EC.text_to_be_present_in_element(
            (By.CSS_SELECTOR, ".post-card .post-caption"), caption_text
        ))

    def test_post_appears_on_own_profile(self, page, user_a, test_image):
        """After uploading, the post thumbnail appears in the profile grid."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("feed")
        page.upload_post(test_image, caption="Profile grid test")

        page.nav_to("profile")
        # Wait for grid to load
        page.wait_css(".post-grid")
        grid_items = page.find_all(".grid-item")
        assert len(grid_items) >= 1


class TestLikes:
    """User can like and unlike posts in the feed."""

    def test_like_button_increments_count(self, page, user_a, test_image):
        """Clicking the like button increases the like counter by 1."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        # Upload a fresh post to like
        page.nav_to("feed")
        page.upload_post(test_image, caption="Like test post")
        page.nav_to("feed")

        card = page.first_post_card()
        like_btn = card.find_element(By.CSS_SELECTOR, ".action-btn")
        count_el = like_btn.find_element(By.CSS_SELECTOR, ".like-count")
        before = int(count_el.text)

        like_btn.click()
        page.sleep(0.8)

        count_el = card.find_element(By.CSS_SELECTOR, ".like-count")
        after = int(count_el.text)
        assert after == before + 1

    def test_like_button_adds_liked_class(self, page, user_a, test_image):
        """After liking, the like button has the 'liked' CSS class."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("feed")
        page.upload_post(test_image, caption="Like class test")
        page.nav_to("feed")

        card = page.first_post_card()
        like_btn = card.find_element(By.CSS_SELECTOR, ".action-btn")

        # Make sure it's not already liked
        if "liked" in (like_btn.get_attribute("class") or ""):
            like_btn.click()   # unlike first
            page.sleep(0.5)

        like_btn.click()
        page.sleep(0.8)

        like_btn = card.find_element(By.CSS_SELECTOR, ".action-btn")
        assert "liked" in (like_btn.get_attribute("class") or "")

    def test_unlike_decrements_count(self, page, user_a, test_image):
        """Clicking a liked post's like button decreases the count by 1."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("feed")
        page.upload_post(test_image, caption="Unlike test")
        page.nav_to("feed")

        card = page.first_post_card()
        like_btn = card.find_element(By.CSS_SELECTOR, ".action-btn")

        # Ensure liked state
        if "liked" not in (like_btn.get_attribute("class") or ""):
            like_btn.click()
            page.sleep(0.5)
            card = page.first_post_card()
            like_btn = card.find_element(By.CSS_SELECTOR, ".action-btn")

        count_before = int(like_btn.find_element(
            By.CSS_SELECTOR, ".like-count").text)
        like_btn.click()
        page.sleep(0.8)
        count_after = int(card.find_element(
            By.CSS_SELECTOR, ".like-count").text)
        assert count_after == count_before - 1


class TestComments:
    """User can open the comments section, add a comment, and delete it."""

    def test_comment_section_opens(self, page, user_a, test_image):
        """Clicking the comment button reveals the comments section."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("feed")
        page.upload_post(test_image, caption="Comment open test")
        page.nav_to("feed")

        card = page.first_post_card()
        comment_btn = card.find_elements(By.CSS_SELECTOR, ".action-btn")[1]
        comment_btn.click()

        # Comments section should become open
        post_id = page.get_first_post_id()
        section = page.wait_css(f"#comments-{post_id}.open")
        assert section.is_displayed()

    def test_add_comment_appears_in_section(self, page, user_a, test_image):
        """A submitted comment appears in the post's comments section."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("feed")
        page.upload_post(test_image, caption="Add comment test")
        page.nav_to("feed")

        # Open comments
        card = page.first_post_card()
        card.find_elements(By.CSS_SELECTOR, ".action-btn")[1].click()

        post_id = page.get_first_post_id()
        page.wait_css(f"#comments-{post_id}.open")

        comment_text = f"Selenium comment {id(self)}"
        page.add_comment(post_id, comment_text)

        # The comment text should appear in the comments section
        page.w.until(EC.text_to_be_present_in_element(
            (By.CSS_SELECTOR, f"#comments-{post_id}"), comment_text
        ))

    def test_comment_count_increments(self, page, user_a, test_image):
        """Adding a comment increases the comment counter on the post card."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("feed")
        page.upload_post(test_image, caption="Comment count test")
        page.nav_to("feed")

        card = page.first_post_card()
        comment_btn = card.find_elements(By.CSS_SELECTOR, ".action-btn")[1]
        count_before = int(comment_btn.find_element(
            By.CSS_SELECTOR, ".comment-count").text)

        comment_btn.click()
        post_id = page.get_first_post_id()
        page.wait_css(f"#comments-{post_id}.open")
        page.add_comment(post_id, "Count test comment")

        page.sleep(1)
        card = page.first_post_card()
        count_after = int(
            card.find_elements(By.CSS_SELECTOR, ".action-btn")[1]
                .find_element(By.CSS_SELECTOR, ".comment-count").text
        )
        assert count_after == count_before + 1

    def test_delete_own_comment(self, page, user_a, test_image):
        """A user can delete their own comment; it disappears from the section."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        page.nav_to("feed")
        page.upload_post(test_image, caption="Delete comment test")
        page.nav_to("feed")

        card = page.first_post_card()
        card.find_elements(By.CSS_SELECTOR, ".action-btn")[1].click()
        post_id = page.get_first_post_id()
        page.wait_css(f"#comments-{post_id}.open")

        comment_text = f"ToDelete_{id(self)}"
        page.add_comment(post_id, comment_text)

        # Wait for the comment to appear
        page.w.until(EC.text_to_be_present_in_element(
            (By.CSS_SELECTOR, f"#comments-{post_id}"), comment_text
        ))

        # Find and click the delete (x) button on the comment row
        section = page.find(f"#comments-{post_id}")
        rows = section.find_elements(By.CSS_SELECTOR, ".comment-row")
        delete_btn = None
        for row in rows:
            if comment_text in row.text:
                btns = row.find_elements(By.CSS_SELECTOR, "button")
                if btns:
                    delete_btn = btns[-1]
                    break

        assert delete_btn is not None, "Delete button not found on comment row"
        delete_btn.click()
        page.sleep(1)

        # Comment text should be gone from the section
        section_text = page.find(f"#comments-{post_id}").text
        assert comment_text not in section_text


class TestPostDeletion:
    """Owner can delete a post from the post detail modal."""

    def test_delete_post_removes_from_feed(self, page, user_a, test_image):
        """Deleting a post via the modal removes it from the feed."""
        if "visible" not in (page.find("#app").get_attribute("class") or ""):
            page.open()
            page.login(user_a["username"], user_a["password"])

        caption = f"ToDelete_post_{id(self)}"
        page.nav_to("feed")
        page.upload_post(test_image, caption=caption)
        page.nav_to("feed")

        # Find the card that has our caption
        page.w.until(EC.text_to_be_present_in_element(
            (By.CSS_SELECTOR, ".post-card"), caption
        ))
        cards = page.find_all(".post-card")
        target_card = None
        for c in cards:
            if caption in c.text:
                target_card = c
                break

        assert target_card is not None

        # Click the owner button
        menu_btn = target_card.find_element(
            By.XPATH, ".//button[contains(text(),'•••')]"
        )
        menu_btn.click()
        page.wait_css("#modal-post.open")

        # Override confirm() so it auto-accepts
        page.js("window.confirm = function() { return true; }")
        delete_btn = page.wait_css("#post-modal-delete-btn")
        delete_btn.click()

        # Modal should close
        page.w.until(EC.invisibility_of_element_located(
            (By.CSS_SELECTOR, "#modal-post.open")
        ))

        # The caption should no longer appear in the feed
        page.sleep(1.5)   # feed reloads
        feed_text = page.find("#feed-container").text
        assert caption not in feed_text
