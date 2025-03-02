# client/python/setup.py

from setuptools import setup, find_packages

setup(
    name="quorum-quest-client",
    version="0.1.0",
    description="Python client for the Quorum Quest leader election service",
    author="Quorum Quest Team",
    author_email="info@quorumquest.io",
    packages=find_packages(),
    install_requires=[
        "grpcio>=1.50.0",
        "protobuf>=4.21.0",
    ],
    python_requires=">=3.7",
    # Add package data to include __init__.py files
    package_data={
        "": ["__init__.py"],
    },
)