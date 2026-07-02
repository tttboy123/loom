class ConfigurationError(Exception):
    """配置层错误。failure_code 用于上层诊断/上报，恒定字符串，禁止自由文本。"""

    def __init__(self, failure_code: str, message: str = ""):
        self.failure_code = failure_code
        super().__init__(message or failure_code)
