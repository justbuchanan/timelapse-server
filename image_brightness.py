#!/usr/bin/env python

import cv2
import numpy as np
import argparse
import os


def parser():
    p = argparse.ArgumentParser()
    p.add_argument('file', type=str, help='path to image file input')
    return p


def calculate_brightness(img):
    avg_color = np.average(np.average(img, axis=0), axis=0)
    num = np.average(avg_color)

    return num / 256.0


def lights_on(brightness):
    light_on_threshold = 70 / 256.0
    return brightness > light_on_threshold


def process_file(file):
    img = cv2.imread(file)

    if img is None:
        raise RuntimeError("Unable to open image file: '%s'" % file)

    return img


def main():
    p = parser()
    args = p.parse_args()

    img = process_file(args.file)
    brightness = calculate_brightness(img)
    print("%.2f" % brightness)


if __name__ == '__main__':
    main()
