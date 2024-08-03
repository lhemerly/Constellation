# Contributing to Constellation

Thank you for considering contributing to Constellation! We welcome contributions from the community and are grateful for your help in improving the project.

## How to Contribute

1. **Open an Issue**:
    - Before starting any work, please open an issue to discuss the change you wish to make. This helps to prevent duplicate work and allows the maintainers to provide feedback early.
    - Describe the bug, feature, or enhancement in detail.

2. **Fork the Repository**:
    - Fork the repository to your own GitHub account and clone it to your local machine.

3. **Create a New Branch**:
    - Create a new branch from the `main` branch for your work:
      ```sh
      git checkout -b your-branch-name
      ```

4. **Write Tests**:
    - Ensure that you write tests for any new features or bug fixes.
    - Tests should cover the functionality thoroughly and edge cases where applicable.

5. **Run Tests**:
    - Make sure all tests pass before submitting your pull request:
      ```sh
      go test ./...
      ```

6. **Lint Your Code**:
    - Use the Go default linter to ensure your code follows the standard Go style:
      ```sh
      go fmt ./...
      ```

7. **Commit Your Changes**:
    - Commit your changes with a clear and concise commit message:
      ```sh
      git add .
      git commit -m "Description of your changes"
      ```

8. **Push Your Branch**:
    - Push your branch to your forked repository:
      ```sh
      git push origin your-branch-name
      ```

9. **Open a Pull Request**:
    - Open a pull request (PR) from your branch to the `main` branch of the Constellation repository.
    - Reference the issue number in your PR description (e.g., "Fixes #123").
    - Provide a detailed description of the changes you made and why.

10. **Code Review**:
    - One of the maintainers will review your pull request.
    - Please address any feedback and make necessary changes.

## Code of Conduct

Please be respectful and considerate in your interactions with others.

## Reporting Issues

If you encounter any bugs or issues, please open an issue on GitHub. Provide as much detail as possible, including steps to reproduce the issue and any relevant information about your environment.

## Contact

If you have any questions or need further assistance, feel free to open an issue or contact the maintainers.

Thank you for contributing to Constellation!
