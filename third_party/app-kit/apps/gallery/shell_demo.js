// Gallery-only enhancement for the synthetic shell workflow. Server validation,
// authorization and saving remain authoritative and work without this script.
(function (window, document) {
  "use strict";
  if (window.AppKitShellDemo) return;
  const editors = new Map();

  function mount(root) {
    for (const form of root.querySelectorAll("form[data-shell-edit]")) {
      if (editors.has(form)) continue;
      let baseline;
      try { baseline = JSON.parse(form.getAttribute("data-shell-baseline")); }
      catch (_) { continue; }
      if (!baseline || typeof baseline !== "object" || Array.isArray(baseline)) continue;
      const fields = Array.from(form.elements).filter((field) => Object.hasOwn(baseline, field.name));
      const status = form.querySelector("[data-shell-draft-status]");
      const button = form.querySelector("[data-shell-save]");
      const resume = form.querySelector("[data-shell-resume]");
      if (!status || !button) continue;
      const editor = { form, dirty: false, submitting: false, outcomeUnknown: false, label: button.textContent };
      const retry = form.getAttribute("data-shell-retry") === "true";
      const changed = (field) =>
        (field.type === "checkbox" ? field.checked : field.value) !== baseline[field.name];
      function announce(message) {
        if (status.textContent !== message) status.textContent = message;
      }
      function update() {
        editor.dirty = fields.some(changed);
        if (editor.submitting) return;
        button.disabled = !editor.dirty && !retry && !editor.outcomeUnknown;
        button.textContent = editor.label;
        if (resume) resume.hidden = true;
        form.removeAttribute("aria-busy");
        announce(editor.outcomeUnknown ?
          "Editing resumed. The server may already have saved. Reload saved values or review any conflict before retrying." :
          editor.dirty ? "Unsaved changes" : retry ? "No unsaved changes. You can retry the failed submission." : "No unsaved changes");
      }
      function resumeEditing(restoreFocus) {
        // Stop the browser navigation, not an already accepted server mutation.
        // A later retry still carries the original optimistic version.
        if (editor.submitting) {
          editor.outcomeUnknown = true;
          if (typeof window.stop === "function") window.stop();
        }
        editor.submitting = false;
        update();
        if (restoreFocus) {
          const target = fields.find((field) => !field.disabled && changed(field)) ||
            fields.find((field) => !field.disabled);
          if (target && typeof target.focus === "function") target.focus();
        }
      }
      const fieldChanged = () => { if (editor.submitting) resumeEditing(false); else update(); };
      form.addEventListener("input", fieldChanged);
      form.addEventListener("change", fieldChanged);
      if (resume) resume.addEventListener("click", () => resumeEditing(document.activeElement === resume));
      form.addEventListener("submit", (event) => {
        if (event.defaultPrevented || !form.checkValidity()) return;
        editor.submitting = true;
        button.disabled = true;
        button.textContent = "Saving changes…";
        if (resume) resume.hidden = false;
        form.setAttribute("aria-busy", "true");
        announce("Saving changes. Your draft remains visible until the server responds.");
      });
      editor.resume = () => {
        // Returning to this document cannot establish a pending write's outcome.
        // In particular, matching the old baseline is not a confirmed clean state.
        if (editor.submitting) editor.outcomeUnknown = true;
        editor.submitting = false;
        update();
      };
      editors.set(form, editor);
      update();
    }
  }

  window.addEventListener("beforeunload", (event) => {
    if (Array.from(editors.values()).some((editor) => editor.form.isConnected &&
      (editor.dirty || editor.outcomeUnknown) && !editor.submitting)) {
      event.preventDefault();
      event.returnValue = "";
    }
  });
  window.addEventListener("pageshow", () => {
    for (const [form, editor] of editors) {
      if (!form.isConnected) editors.delete(form);
      else editor.resume();
    }
  });
  window.AppKitShellDemo = Object.freeze({ mount });
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", () => mount(document), { once: true });
  else mount(document);
})(window, document);
