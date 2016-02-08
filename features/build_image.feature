Feature: build an image

Scenario: built Ubuntu base image
  Given an initialized Jetpack installation
  And no image named "3ofcoins.net/ubuntu,os=linux"
  When I cd to "images/ubuntu"
  And I run: make clean
  And I run: make
  Then there is an image named "3ofcoins.net/ubuntu,os=linux"

