# Contributing to Guardrail Studio

Thank you for your interest in contributing to Guardrail Studio! This guide will help you get started.

## 🎯 Ways to Contribute

### 1. Add New Templates

The most valuable contribution is adding new guardrail templates for different use cases.

**Template Format:**

```yaml
# templates/your-template.yaml
id: your_template_id
name: Human Readable Name
version: 1.0.0
type: smart_guardrail

description: |
  Clear description of what this guardrail blocks and allows.
  Be specific about the boundary between safe and unsafe content.

calibration:
  t_allow: 0.25    # Tune based on testing
  t_block: 0.60

safe_examples:
  - "Example 1 that should be ALLOWED"
  - "Example 2 that should be ALLOWED"
  # ... minimum 10 examples

unsafe_examples:
  - "Example 1 that should be BLOCKED"
  - "Example 2 that should be BLOCKED"
  # ... minimum 10 examples

metadata:
  category: your-category
  risk_level: low|medium|high|critical
  compliance:
    - Relevant frameworks
  tags:
    - relevant
    - tags
  author: Your Name
  created: YYYY-MM-DD
```

**Quality Guidelines:**

1. **Minimum 10 examples each** for safe and unsafe categories
2. **Clear boundary** - examples should be unambiguous
3. **Diverse examples** - cover different phrasings and scenarios
4. **Test locally** - verify accuracy before submitting

### 2. Improve Existing Templates

- Add more examples to improve accuracy
- Fix edge cases
- Update for new regulations or best practices

### 3. UI Improvements

- Accessibility improvements
- New features
- Bug fixes
- Performance optimizations

### 4. Documentation

- Improve README
- Add usage examples
- Write tutorials

## 📝 Pull Request Process

1. **Fork** the repository
2. **Create a branch** for your feature: `git checkout -b feature/my-template`
3. **Make your changes**
4. **Test locally** using `python3 -m http.server 8090`
5. **Commit** with a clear message: `git commit -m "Add healthcare HIPAA template"`
6. **Push** to your fork: `git push origin feature/my-template`
7. **Open a Pull Request** with a description of your changes

## 🧪 Testing Your Template

1. Start the local server:
   ```bash
   python3 -m http.server 8090
   ```

2. Open http://localhost:8090

3. Import your template by clicking "View all →" and uploading

4. Click "Generate Guardrail"

5. Test with various inputs using the Test Playground

6. Verify accuracy metrics meet quality standards:
   - **Accuracy**: > 70%
   - **Recall**: > 80% (to minimize false negatives)

## 📜 Contributor License Agreement

By contributing to this repository, you agree that:

1. Your contributions are submitted under the Apache 2.0 license
2. You grant EthicalZen the right to use your contributions commercially
3. You have the right to submit the contribution

## 🏆 Recognition

All contributors are recognized in:
- The project README
- The template files (author field)
- Our website's contributor page

## 💬 Getting Help

- **Discord**: [discord.gg/ethicalzen](https://discord.gg/ethicalzen)
- **GitHub Issues**: For bugs and feature requests
- **Email**: hello@ethicalzen.ai

## 📋 Template Ideas

Looking for ideas? Here are some templates we'd love to see:

- [ ] EU AI Act compliance
- [ ] GDPR data privacy
- [ ] Age-appropriate content
- [ ] Political neutrality
- [ ] Hate speech detection
- [ ] Self-harm prevention
- [ ] Misinformation detection
- [ ] Copyright protection
- [ ] Industry-specific (insurance, real estate, etc.)

---

Thank you for helping make AI safer! 🛡️

